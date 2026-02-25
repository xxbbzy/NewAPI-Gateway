package model

import (
	"reflect"
	"testing"
)

func TestBuildModelCatalogDeterministicCanonicalForAliasConflicts(t *testing.T) {
	routes := []ModelRoute{
		{Id: 1, ModelName: "target-model-20250101", ProviderId: 1},
		{Id: 2, ModelName: "target-model-20250101", ProviderId: 2},
	}
	reverseLookups := map[int]providerModelAliasReverseLookup{
		1: buildProviderModelAliasReverseLookup(map[string]string{"zeta": "target-model-20250101"}),
		2: buildProviderModelAliasReverseLookup(map[string]string{"alpha": "target-model-20250101"}),
	}

	snapshotA := buildModelCatalogSnapshot(routes, reverseLookups)
	reversedRoutes := []ModelRoute{routes[1], routes[0]}
	snapshotB := buildModelCatalogSnapshot(reversedRoutes, reverseLookups)

	entryA, ok := snapshotA.resolve("target-model-20250101")
	if !ok {
		t.Fatalf("expected target model to be resolved in snapshot A")
	}
	entryB, ok := snapshotB.resolve("target-model-20250101")
	if !ok {
		t.Fatalf("expected target model to be resolved in snapshot B")
	}

	if entryA.CanonicalModel != "alpha" {
		t.Fatalf("expected canonical model alpha, got %s", entryA.CanonicalModel)
	}
	if entryB.CanonicalModel != entryA.CanonicalModel {
		t.Fatalf("expected deterministic canonical model, A=%s B=%s", entryA.CanonicalModel, entryB.CanonicalModel)
	}
	if !reflect.DeepEqual(entryA.Aliases, entryB.Aliases) {
		t.Fatalf("expected deterministic alias ordering, A=%v B=%v", entryA.Aliases, entryB.Aliases)
	}
}

func TestBuildModelCatalogResolveAliasAndTarget(t *testing.T) {
	routes := []ModelRoute{
		{Id: 1, ModelName: "bbbxxxcccddd", ProviderId: 1},
	}
	reverseLookups := map[int]providerModelAliasReverseLookup{
		1: buildProviderModelAliasReverseLookup(map[string]string{"aaa": "bbbxxxcccddd"}),
	}

	snapshot := buildModelCatalogSnapshot(routes, reverseLookups)

	aliasEntry, ok := snapshot.resolve("aaa")
	if !ok {
		t.Fatalf("expected alias aaa to resolve")
	}
	if aliasEntry.CanonicalModel != "aaa" {
		t.Fatalf("expected canonical aaa from alias, got %s", aliasEntry.CanonicalModel)
	}

	targetEntry, ok := snapshot.resolve("bbbxxxcccddd")
	if !ok {
		t.Fatalf("expected route target to resolve")
	}
	if targetEntry.CanonicalModel != "aaa" {
		t.Fatalf("expected canonical aaa from target, got %s", targetEntry.CanonicalModel)
	}

	normalizedEntry, ok := snapshot.resolve("BBBXXXCCCDDD")
	if !ok {
		t.Fatalf("expected uppercase target to resolve")
	}
	if normalizedEntry.CanonicalModel != "aaa" {
		t.Fatalf("expected canonical aaa from uppercase target, got %s", normalizedEntry.CanonicalModel)
	}
}
