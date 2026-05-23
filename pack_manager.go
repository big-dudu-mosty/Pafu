package main

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

var (
	packRegistryMu sync.RWMutex
	packRegistry   map[string]*soundPack
	activePackID   string
	activeRuntime  *activePackRuntime
)

type packInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Mode      string `json:"mode"`
	Source    string `json:"source"`
	BuiltIn   bool   `json:"built_in"`
	Active    bool   `json:"active"`
	FileCount int    `json:"file_count"`
	Path      string `json:"path,omitempty"`
}

type activePackRuntime struct {
	mu       sync.RWMutex
	id       string
	pack     *soundPack
	tracker  *slapTracker
	cooldown time.Duration
}

func playModeString(mode playMode) string {
	switch mode {
	case modeEscalation:
		return "escalation"
	default:
		return "random"
	}
}

func packSource(pack *soundPack) string {
	if pack.source != "" {
		return pack.source
	}
	if pack.custom {
		return "custom"
	}
	return "builtin"
}

func packDisplayName(id string) string {
	switch id {
	case "pain":
		return "Pain"
	case "sexy":
		return "Sexy"
	case "halo":
		return "Halo"
	case "lizard":
		return "Lizard"
	case "custom":
		return "Custom"
	case "custom-files":
		return "Custom Files"
	default:
		return id
	}
}

func toPackInfo(id string, pack *soundPack, active bool) packInfo {
	name := pack.displayName
	if name == "" {
		name = packDisplayName(id)
	}
	return packInfo{
		ID:        id,
		Name:      name,
		Mode:      playModeString(pack.mode),
		Source:    packSource(pack),
		BuiltIn:   packSource(pack) == "builtin",
		Active:    active,
		FileCount: len(pack.files),
		Path:      pack.dir,
	}
}

func buildPackRegistry(initialID string, initialPack *soundPack, cooldown time.Duration) error {
	registry := map[string]*soundPack{
		"pain":   {name: "pain", fs: painAudio, dir: "audio/pain", mode: modeRandom},
		"sexy":   {name: "sexy", fs: sexyAudio, dir: "audio/sexy", mode: modeEscalation},
		"halo":   {name: "halo", fs: haloAudio, dir: "audio/halo", mode: modeRandom},
		"lizard": {name: "lizard", fs: lizardAudio, dir: "audio/lizard", mode: modeEscalation},
	}

	for id, pack := range registry {
		if id == initialID {
			// Preserve the already validated/loaded initial pack pointer.
			registry[id] = initialPack
			continue
		}
		if err := pack.loadFiles(); err != nil {
			return fmt.Errorf("loading %s audio: %w", id, err)
		}
	}

	if initialPack.custom {
		registry[initialID] = initialPack
	}
	userPacks, err := loadUserPacks()
	if err != nil {
		return fmt.Errorf("loading user packs: %w", err)
	}
	for id, pack := range userPacks {
		registry[id] = pack
	}

	packRegistryMu.Lock()
	packRegistry = registry
	packRegistryMu.Unlock()

	initial, ok := registry[initialID]
	if !ok {
		return fmt.Errorf("initial pack not registered: %s", initialID)
	}
	setActivePack(initialID, initial, cooldown)
	return nil
}

func registerPack(id string, pack *soundPack) {
	packRegistryMu.Lock()
	defer packRegistryMu.Unlock()
	if packRegistry == nil {
		packRegistry = make(map[string]*soundPack)
	}
	packRegistry[id] = pack
}

func unregisterPack(id string) {
	packRegistryMu.Lock()
	defer packRegistryMu.Unlock()
	delete(packRegistry, id)
}

func setActivePack(id string, pack *soundPack, cooldown time.Duration) {
	if activeRuntime == nil {
		activeRuntime = &activePackRuntime{cooldown: cooldown}
	}
	activeRuntime.mu.Lock()
	activeRuntime.id = id
	activeRuntime.pack = pack
	activeRuntime.tracker = newSlapTracker(pack, cooldown)
	activeRuntime.cooldown = cooldown
	activeRuntime.mu.Unlock()

	activePackID = id
	currentMode = pack.name
}

func activePackSnapshot() (string, *soundPack, *slapTracker) {
	if activeRuntime == nil {
		return "", nil, nil
	}
	activeRuntime.mu.RLock()
	defer activeRuntime.mu.RUnlock()
	return activeRuntime.id, activeRuntime.pack, activeRuntime.tracker
}

func listPacks() []packInfo {
	packRegistryMu.RLock()
	defer packRegistryMu.RUnlock()

	ids := make([]string, 0, len(packRegistry))
	for id := range packRegistry {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	packs := make([]packInfo, 0, len(packRegistry))
	for _, id := range ids {
		pack := packRegistry[id]
		packs = append(packs, toPackInfo(id, pack, id == activePackID))
	}
	return packs
}

func activatePack(id string) (packInfo, error) {
	packRegistryMu.RLock()
	pack, ok := packRegistry[id]
	packRegistryMu.RUnlock()
	if !ok {
		return packInfo{}, fmt.Errorf("pack not found: %s", id)
	}

	cooldown := time.Duration(cooldownMs) * time.Millisecond
	if activeRuntime != nil {
		activeRuntime.mu.RLock()
		cooldown = activeRuntime.cooldown
		activeRuntime.mu.RUnlock()
	}
	setActivePack(id, pack, cooldown)

	info := toPackInfo(id, pack, true)
	publishBus("pack-changed", map[string]interface{}{
		"active_pack_id": id,
		"pack":           info,
	})
	return info, nil
}
