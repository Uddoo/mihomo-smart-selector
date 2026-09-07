package scan

import (
	"context"
	"fmt"
	"strings"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func (m *Manager) bindingScope() string {
	return strings.TrimRight(m.currentConfig().Mihomo.Controller, "/")
}

func (m *Manager) Services(ctx context.Context) (model.ServiceCatalog, error) {
	catalog := model.ServiceCatalog{Profiles: []model.ProbeProfileSummary{}, Suggestions: map[string]string{}, DefaultProfileID: m.currentConfig().Scanner.DefaultProbeProfile}
	for _, p := range m.currentConfig().Scanner.ProbeProfiles {
		catalog.Profiles = append(catalog.Profiles, m.profileSummary(p))
	}
	if len(catalog.Profiles) == 0 {
		p, _ := m.currentConfig().ResolveProbeProfile("")
		catalog.Profiles = append(catalog.Profiles, m.profileSummary(p))
		catalog.DefaultProfileID = p.ID
	}
	groups, err := m.Groups(ctx)
	if err != nil {
		return catalog, err
	}
	present := map[string]bool{}
	for _, group := range groups {
		present[group.Name] = true
		if id := m.currentConfig().SuggestedProfile(group.Name); id != "" {
			catalog.Suggestions[group.Name] = id
		}
	}
	catalog.Bindings, err = m.store.Bindings(ctx, m.bindingScope())
	if err != nil {
		return catalog, err
	}
	for i := range catalog.Bindings {
		item := &catalog.Bindings[i]
		item.Status = "valid"
		if !present[item.Group] {
			item.Status = "group_missing"
		} else if _, err := m.currentConfig().ProbeProfileByID(item.ProfileID); err != nil {
			item.Status = "profile_missing"
		}
	}
	return catalog, nil
}

func (m *Manager) SetBinding(ctx context.Context, item model.ServiceBinding) error {
	if strings.TrimSpace(item.Group) == "" {
		return fmt.Errorf("group is required")
	}
	if item.ProfileID != "" {
		profile, err := m.currentConfig().ProbeProfileByID(item.ProfileID)
		if err != nil {
			return err
		}
		item.ProfileID = profile.ID
		groups, err := m.Groups(ctx)
		if err != nil {
			return err
		}
		found := false
		for _, group := range groups {
			if group.Name == item.Group {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("group %q is no longer a selector; refresh the group list", item.Group)
		}
	}
	return m.store.SetBinding(ctx, m.bindingScope(), item)
}

func (m *Manager) resolveService(ctx context.Context, request model.ScanRequest) (config.ProbeProfile, error) {
	if request.ProfileID != "" {
		return m.currentConfig().ProbeProfileByID(request.ProfileID)
	}
	bindings, err := m.store.Bindings(ctx, m.bindingScope())
	if err != nil {
		return config.ProbeProfile{}, err
	}
	for _, item := range bindings {
		if item.Group == request.TargetGroup {
			return m.currentConfig().ProbeProfileByID(item.ProfileID)
		}
	}
	return m.currentConfig().ResolveProbeProfile(request.TargetGroup)
}
