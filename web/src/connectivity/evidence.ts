import type {ResultView, Target} from './engine'

export function evidenceLabel(result: ResultView) {
  const successes = result.samples.filter(sample => sample.outcome === 'success')
  if (successes.length) {
    if (successes.every(sample => sample.level === 'verified')) return '验证通过'
    if (successes.every(sample => sample.level === 'resource')) return '资源可达'
    return '可达'
  }
  if (result.samples.some(sample => sample.outcome === 'mismatch')) return '响应不符'
  if (result.samples.some(sample => sample.outcome === 'unverifiable')) return '无法验证'
  return ''
}

export const targetKinds = {resource: '静态资源', connectivity: '专用连通性接口', api: '公开只读接口', diagnostic: '网络诊断接口', web: '网站入口'} as const
export const bodyRules = {none: '不检查正文', empty: '正文为空', contains: '包含指定内容', json: 'JSON 结构符合预期'} as const
export const failureReasons = {
  csp: '页面安全策略拦截了探测请求。',
  browser: '跨域验证未完成：可能是跨域限制、重定向或网络错误，浏览器未提供具体原因。',
  status: 'HTTP 状态码不符合预期。', body: '响应正文不符合预期。', 'content-type': '响应类型不符合预期。',
  'response-too-large': '响应超过 64 KiB，已停止读取。',
} as const

export function validationScope(target: Target) {
  return target.requestMode === 'no-cors' ? '浏览器只能确认收到响应，不能核对 HTTP 状态或正文。'
    : '浏览器校验可读取的状态和响应规则；通过仅代表此入口符合预期。'
}
