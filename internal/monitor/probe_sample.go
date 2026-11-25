package monitor

import (
	"context"
	"errors"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"github.com/Uddoo/mihomo-smart-selector/internal/monitor/probe"
)

// probeNode fills the evidence for one sampling slot. A cancelled slot has no evidence to persist.
func (m *TaskRuntime) probeNode(ctx context.Context, node model.MonitorNode, profile config.ProbeProfile, sample *model.MonitorSample) bool {
	sample.Outcome = "success"
	for _, target := range profile.Probes {
		if !m.takeBudget(m.now()) {
			setProbeFailure(sample, "unknown", "budget", "已达到后台请求预算，本次未完成", "自动切换等待探测预算")
			break
		}
		delay, err := m.source.MonitorDelay(ctx, node, target)
		switch {
		case ctx.Err() != nil || errors.Is(err, context.Canceled):
			return false
		case errors.Is(err, probe.ErrBusy):
			setProbeFailure(sample, "unknown", "busy", "手动扫描占用探测额度，本次漏测", "探测额度或 Controller 不可用，暂不切换")
		case errors.Is(err, probe.ErrAuthentication), errors.Is(err, probe.ErrControllerUnavailable):
			setProbeFailure(sample, "unknown", "controller", "Controller 无法完成请求，未归因于节点", "探测额度或 Controller 不可用，暂不切换")
		case err != nil || delay <= 0:
			// Bound series assign node_path on write. Legacy unbound samples keep the empty code.
			setProbeFailure(sample, "failure", "", "节点路径探测失败或超时；未定位具体故障位置", "自动切换前复测失败")
		default:
			sample.DelayMS = max(sample.DelayMS, delay)
			continue
		}
		break
	}
	return true
}

func setProbeFailure(sample *model.MonitorSample, outcome, code, samplingReason, failoverReason string) {
	sample.Outcome, sample.ReasonCode, sample.Reason = outcome, code, samplingReason
	if sample.Kind == "failover" {
		sample.Reason = failoverReason
	}
}
