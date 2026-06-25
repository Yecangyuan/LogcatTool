package model

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/Yecangyuan/LogcatTool/internal/config"
	"github.com/Yecangyuan/LogcatTool/internal/logentry"
)

type profileNameMode int

const (
	profileNameSave profileNameMode = iota
	profileNameRename
)

func profileFromSnapshot(name string, snapshot logentry.Snapshot) config.Profile {
	return config.Profile{
		Name:          strings.TrimSpace(name),
		MinLevel:      snapshot.MinLevel.Char(),
		Package:       snapshot.Package,
		Process:       snapshot.Process,
		Tag:           snapshot.Tag,
		TagExclude:    snapshot.TagExclude,
		PID:           snapshot.PID,
		SearchText:    snapshot.SearchText,
		IsRegex:       snapshot.IsRegex,
		CrashOnly:     snapshot.CrashOnly,
		TimeWindowSec: int(snapshot.TimeWindow.Seconds()),
	}
}

func snapshotFromProfile(profile config.Profile) logentry.Snapshot {
	return logentry.Snapshot{
		MinLevel:   logentry.ParseLevelString(profile.MinLevel),
		Package:    profile.Package,
		Process:    profile.Process,
		Tag:        profile.Tag,
		TagExclude: profile.TagExclude,
		PID:        profile.PID,
		SearchText: profile.SearchText,
		IsRegex:    profile.IsRegex,
		CrashOnly:  profile.CrashOnly,
		TimeWindow: time.Duration(profile.TimeWindowSec) * time.Second,
	}
}

func (m *AppModel) saveProfile(name string) {
	profile := profileFromSnapshot(name, m.filter.Snapshot())
	if profile.Name == "" {
		m.statusMsg = "请输入配置名称"
		return
	}
	for i := range m.cfg.Profiles {
		if m.cfg.Profiles[i].Name == profile.Name {
			m.cfg.Profiles[i] = profile
			m.statusMsg = fmt.Sprintf("已更新配置: %s", profile.Name)
			return
		}
	}
	m.cfg.Profiles = append(m.cfg.Profiles, profile)
	m.statusMsg = fmt.Sprintf("已保存配置: %s", profile.Name)
}

func (m *AppModel) applyProfile(idx int) {
	if idx < 0 || idx >= len(m.cfg.Profiles) {
		m.statusMsg = "配置不存在"
		return
	}
	profile := m.cfg.Profiles[idx]
	m.filter.ApplySnapshot(snapshotFromProfile(profile))
	m.refilter()
	m.statusMsg = fmt.Sprintf("已应用配置: %s", profile.Name)
}

func (m *AppModel) renameProfile(idx int, name string) {
	if idx < 0 || idx >= len(m.cfg.Profiles) {
		m.statusMsg = "配置不存在"
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		m.statusMsg = "请输入配置名称"
		return
	}
	m.cfg.Profiles[idx].Name = name
	m.statusMsg = fmt.Sprintf("已重命名配置: %s", name)
}

func (m *AppModel) deleteProfile(idx int) {
	if idx < 0 || idx >= len(m.cfg.Profiles) {
		m.statusMsg = "配置不存在"
		return
	}
	name := m.cfg.Profiles[idx].Name
	m.cfg.Profiles = append(m.cfg.Profiles[:idx], m.cfg.Profiles[idx+1:]...)
	m.statusMsg = fmt.Sprintf("已删除配置: %s", name)
}

func (m AppModel) applySelectedProfileCmd() tea.Cmd {
	if m.profileSelection < 0 || m.profileSelection >= len(m.cfg.Profiles) {
		return nil
	}
	m.applyProfile(m.profileSelection)
	if m.filePath == "" && (m.filter.Package != "" || m.filter.Process != "") {
		return loadPackagePIDs(m.adbPath, m.currentDeviceSerial())
	}
	return nil
}
