package model

import (
	"NewAPI-Gateway/common"
	"sort"
	"strings"
)

// UnifiedModelCatalogItem represents one canonical model view used by list/query APIs.
type UnifiedModelCatalogItem struct {
	CanonicalModel string   `json:"canonical_model"`
	Aliases        []string `json:"aliases"`
	RouteTargets   []string `json:"route_targets"`
}

type modelCatalogSnapshot struct {
	entries      []UnifiedModelCatalogItem
	byCanonical  map[string]int
	byExact      map[string]int
	byNormalized map[string]int
	byVersionKey map[string]int
}

type catalogNameCandidate struct {
	name string
	rank int
}

type modelCatalogBuildBucket struct {
	canonicalName string
	routeTargets  map[string]string
	aliases       map[string]string
	candidates    map[string]catalogNameCandidate
}

func newModelCatalogBuildBucket() *modelCatalogBuildBucket {
	return &modelCatalogBuildBucket{
		routeTargets: make(map[string]string),
		aliases:      make(map[string]string),
		candidates:   make(map[string]catalogNameCandidate),
	}
}

func (b *modelCatalogBuildBucket) addRouteTarget(value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return
	}
	lower := strings.ToLower(trimmed)
	if existing, ok := b.routeTargets[lower]; !ok || compareTextStable(trimmed, existing) < 0 {
		b.routeTargets[lower] = trimmed
	}
}

func (b *modelCatalogBuildBucket) addAlias(value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return
	}
	lower := strings.ToLower(trimmed)
	if existing, ok := b.aliases[lower]; !ok || compareTextStable(trimmed, existing) < 0 {
		b.aliases[lower] = trimmed
	}
}

func (b *modelCatalogBuildBucket) addCandidate(value string, rank int) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return
	}
	lower := strings.ToLower(trimmed)
	current, ok := b.candidates[lower]
	if !ok || rank < current.rank || (rank == current.rank && compareTextStable(trimmed, current.name) < 0) {
		b.candidates[lower] = catalogNameCandidate{name: trimmed, rank: rank}
	}
}

func compareTextStable(a string, b string) int {
	al := strings.ToLower(strings.TrimSpace(a))
	bl := strings.ToLower(strings.TrimSpace(b))
	if al < bl {
		return -1
	}
	if al > bl {
		return 1
	}
	if strings.TrimSpace(a) < strings.TrimSpace(b) {
		return -1
	}
	if strings.TrimSpace(a) > strings.TrimSpace(b) {
		return 1
	}
	return 0
}

func chooseCanonicalName(bucket *modelCatalogBuildBucket) string {
	if bucket == nil {
		return ""
	}

	if len(bucket.candidates) == 0 {
		for _, alias := range sortedMapValues(bucket.aliases) {
			return alias
		}
		for _, target := range sortedMapValues(bucket.routeTargets) {
			return target
		}
		return ""
	}

	candidates := make([]catalogNameCandidate, 0, len(bucket.candidates))
	for _, candidate := range bucket.candidates {
		candidates = append(candidates, candidate)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].rank != candidates[j].rank {
			return candidates[i].rank < candidates[j].rank
		}
		cmp := compareTextStable(candidates[i].name, candidates[j].name)
		return cmp < 0
	})
	return strings.TrimSpace(candidates[0].name)
}

func sortedMapValues(values map[string]string) []string {
	if len(values) == 0 {
		return []string{}
	}
	items := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		items = append(items, trimmed)
	}
	sort.Slice(items, func(i, j int) bool {
		cmp := compareTextStable(items[i], items[j])
		return cmp < 0
	})
	return items
}

func newModelCatalogSnapshot() modelCatalogSnapshot {
	return modelCatalogSnapshot{
		entries:      []UnifiedModelCatalogItem{},
		byCanonical:  make(map[string]int),
		byExact:      make(map[string]int),
		byNormalized: make(map[string]int),
		byVersionKey: make(map[string]int),
	}
}

func (s *modelCatalogSnapshot) registerLookup(entryIdx int, candidate string) {
	if s == nil {
		return
	}
	trimmed := strings.TrimSpace(candidate)
	if trimmed == "" {
		return
	}
	lower := strings.ToLower(trimmed)
	if _, exists := s.byExact[lower]; !exists {
		s.byExact[lower] = entryIdx
	}
	if normalized := common.NormalizeModelName(trimmed); normalized != "" {
		if _, exists := s.byNormalized[normalized]; !exists {
			s.byNormalized[normalized] = entryIdx
		}
		if versionKey := common.ToVersionAgnosticKey(normalized); versionKey != "" {
			if _, exists := s.byVersionKey[versionKey]; !exists {
				s.byVersionKey[versionKey] = entryIdx
			}
		}
	}
}

func (s modelCatalogSnapshot) listEntries() []UnifiedModelCatalogItem {
	if len(s.entries) == 0 {
		return []UnifiedModelCatalogItem{}
	}
	copied := make([]UnifiedModelCatalogItem, 0, len(s.entries))
	for _, entry := range s.entries {
		item := UnifiedModelCatalogItem{
			CanonicalModel: entry.CanonicalModel,
			Aliases:        append([]string{}, entry.Aliases...),
			RouteTargets:   append([]string{}, entry.RouteTargets...),
		}
		copied = append(copied, item)
	}
	return copied
}

func (s modelCatalogSnapshot) canonicalModels() []string {
	if len(s.entries) == 0 {
		return []string{}
	}
	models := make([]string, 0, len(s.entries))
	for _, entry := range s.entries {
		canonical := strings.TrimSpace(entry.CanonicalModel)
		if canonical == "" {
			continue
		}
		models = append(models, canonical)
	}
	return models
}

func (s modelCatalogSnapshot) resolve(modelName string) (UnifiedModelCatalogItem, bool) {
	trimmed := strings.TrimSpace(modelName)
	if trimmed == "" || len(s.entries) == 0 {
		return UnifiedModelCatalogItem{}, false
	}

	lower := strings.ToLower(trimmed)
	if idx, ok := s.byCanonical[lower]; ok && idx >= 0 && idx < len(s.entries) {
		return s.entries[idx], true
	}
	if idx, ok := s.byExact[lower]; ok && idx >= 0 && idx < len(s.entries) {
		return s.entries[idx], true
	}

	normalized := common.NormalizeModelName(trimmed)
	if normalized != "" {
		if idx, ok := s.byNormalized[normalized]; ok && idx >= 0 && idx < len(s.entries) {
			return s.entries[idx], true
		}
		if versionKey := common.ToVersionAgnosticKey(normalized); versionKey != "" {
			if idx, ok := s.byVersionKey[versionKey]; ok && idx >= 0 && idx < len(s.entries) {
				return s.entries[idx], true
			}
		}
	}

	return UnifiedModelCatalogItem{}, false
}

func buildModelCatalogSnapshot(routes []ModelRoute, reverseLookups map[int]providerModelAliasReverseLookup) modelCatalogSnapshot {
	snapshot := newModelCatalogSnapshot()
	if len(routes) == 0 {
		return snapshot
	}

	semanticBuckets := make(map[string]*modelCatalogBuildBucket)
	for _, route := range routes {
		routeName := strings.TrimSpace(route.ModelName)
		if routeName == "" {
			continue
		}

		routeLower := strings.ToLower(routeName)
		routeNormalized := common.NormalizeModelName(routeName)
		semanticKey := routeLower
		if routeNormalized != "" {
			semanticKey = routeNormalized
		}

		bucket, exists := semanticBuckets[semanticKey]
		if !exists {
			bucket = newModelCatalogBuildBucket()
			semanticBuckets[semanticKey] = bucket
		}

		bucket.addRouteTarget(routeName)
		bucket.addAlias(routeName)
		bucket.addCandidate(routeName, 3)
		if routeNormalized != "" {
			bucket.addAlias(routeNormalized)
			bucket.addCandidate(routeNormalized, 2)
		}

		if reverseLookup, ok := reverseLookups[route.ProviderId]; ok {
			if source, matched := reverseLookup.ResolveByTarget(routeName); matched {
				bucket.addAlias(source)
				bucket.addCandidate(source, 1)
				if sourceNormalized := common.NormalizeModelName(source); sourceNormalized != "" {
					bucket.addAlias(sourceNormalized)
				}
			}
		}
	}

	if len(semanticBuckets) == 0 {
		return snapshot
	}

	semanticKeys := make([]string, 0, len(semanticBuckets))
	for key := range semanticBuckets {
		semanticKeys = append(semanticKeys, key)
	}
	sort.Strings(semanticKeys)

	canonicalBuckets := make(map[string]*modelCatalogBuildBucket)
	for _, key := range semanticKeys {
		bucket := semanticBuckets[key]
		canonical := chooseCanonicalName(bucket)
		if canonical == "" {
			continue
		}
		canonicalKey := strings.ToLower(strings.TrimSpace(canonical))
		merged, exists := canonicalBuckets[canonicalKey]
		if !exists {
			merged = newModelCatalogBuildBucket()
			merged.canonicalName = canonical
			canonicalBuckets[canonicalKey] = merged
		}
		if merged.canonicalName == "" || compareTextStable(canonical, merged.canonicalName) < 0 {
			merged.canonicalName = canonical
		}

		merged.addAlias(canonical)
		for _, alias := range sortedMapValues(bucket.aliases) {
			merged.addAlias(alias)
		}
		for _, target := range sortedMapValues(bucket.routeTargets) {
			merged.addRouteTarget(target)
		}
	}

	canonicalKeys := make([]string, 0, len(canonicalBuckets))
	for key := range canonicalBuckets {
		canonicalKeys = append(canonicalKeys, key)
	}
	sort.Strings(canonicalKeys)

	for _, canonicalKey := range canonicalKeys {
		bucket := canonicalBuckets[canonicalKey]
		canonical := strings.TrimSpace(bucket.canonicalName)
		if canonical == "" {
			canonical = canonicalKey
		}
		bucket.addAlias(canonical)

		entry := UnifiedModelCatalogItem{
			CanonicalModel: canonical,
			Aliases:        sortedMapValues(bucket.aliases),
			RouteTargets:   sortedMapValues(bucket.routeTargets),
		}
		entryIdx := len(snapshot.entries)
		snapshot.entries = append(snapshot.entries, entry)
		snapshot.byCanonical[strings.ToLower(canonical)] = entryIdx
		snapshot.registerLookup(entryIdx, canonical)
		for _, alias := range entry.Aliases {
			snapshot.registerLookup(entryIdx, alias)
		}
		for _, target := range entry.RouteTargets {
			snapshot.registerLookup(entryIdx, target)
		}
	}

	return snapshot
}

// GetUnifiedModelCatalog returns all canonical models derived from enabled routes.
func GetUnifiedModelCatalog() ([]UnifiedModelCatalogItem, error) {
	snapshot, err := getRoutingStaticSnapshot()
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return []UnifiedModelCatalogItem{}, nil
	}
	return snapshot.modelCatalog.listEntries(), nil
}

// ResolveModelCatalogEntry resolves a user input model name to one catalog entry.
func ResolveModelCatalogEntry(modelName string) (*UnifiedModelCatalogItem, bool, error) {
	snapshot, err := getRoutingStaticSnapshot()
	if err != nil {
		return nil, false, err
	}
	if snapshot == nil {
		return nil, false, nil
	}
	entry, ok := snapshot.modelCatalog.resolve(modelName)
	if !ok {
		return nil, false, nil
	}
	copied := UnifiedModelCatalogItem{
		CanonicalModel: entry.CanonicalModel,
		Aliases:        append([]string{}, entry.Aliases...),
		RouteTargets:   append([]string{}, entry.RouteTargets...),
	}
	return &copied, true, nil
}

// ResolveCanonicalModelName resolves any supported alias/target into canonical name.
func ResolveCanonicalModelName(modelName string) (string, bool, error) {
	entry, ok, err := ResolveModelCatalogEntry(modelName)
	if err != nil || !ok || entry == nil {
		return "", false, err
	}
	canonical := strings.TrimSpace(entry.CanonicalModel)
	if canonical == "" {
		return "", false, nil
	}
	return canonical, true, nil
}
