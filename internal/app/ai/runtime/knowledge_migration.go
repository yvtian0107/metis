package runtime

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"gorm.io/gorm"
)

// MigrateKnowledgeData performs a one-time migration from the legacy
// ai_knowledge_bases / kb_id-based sources to the new unified asset model.
// It is idempotent: if assets already exist, it skips.
//
// Migration logic:
//  1. Each legacy KnowledgeBase → a KnowledgeAsset (category=kg, type=concept_map)
//  2. Each KnowledgeSource that has a kb_id → a row in ai_knowledge_asset_sources
//  3. The FalkorDB graph rename (kb_<id> → kg_<id>) must be done manually by the admin
//     since FalkorDB doesn't support graph rename in a SQL migration.
func MigrateKnowledgeData(db *gorm.DB) error {
	// Check if legacy table exists
	if !db.Migrator().HasTable("ai_knowledge_bases") {
		slog.Debug("knowledge migration: ai_knowledge_bases table not found, skipping")
		return nil
	}

	// Check if we already migrated (assets exist with source from legacy)
	var assetCount int64
	db.Model(&KnowledgeAsset{}).Where("category = ?", AssetCategoryKG).Count(&assetCount)
	if assetCount > 0 {
		slog.Debug("knowledge migration: assets already exist, skipping", "count", assetCount)
		return nil
	}

	// Load legacy knowledge bases
	var legacyKBs []KnowledgeBase
	if err := db.Find(&legacyKBs).Error; err != nil {
		return fmt.Errorf("load legacy knowledge bases: %w", err)
	}
	if len(legacyKBs) == 0 {
		slog.Debug("knowledge migration: no legacy knowledge bases found")
		return nil
	}

	slog.Info("knowledge migration: starting", "legacy_kbs", len(legacyKBs))

	return db.Transaction(func(tx *gorm.DB) error {
		for _, kb := range legacyKBs {
			asset := KnowledgeAsset{
				Name:                kb.Name,
				Description:         kb.Description,
				Category:            AssetCategoryKG,
				Type:                KGTypeConceptMap,
				Status:              mapLegacyStatus(kb.CompileStatus),
				CompileModelID:      kb.CompileModelID,
				EmbeddingProviderID: kb.EmbeddingProviderID,
				EmbeddingModelID:    kb.EmbeddingModelID,
				AutoBuild:           kb.AutoCompile,
				SourceCount:         kb.SourceCount,
				BuiltAt:             kb.CompiledAt,
			}

			// Migrate compile config to graph config
			if kb.CompileConfigData != "" {
				var legacyCfg CompileConfig
				if err := json.Unmarshal([]byte(kb.CompileConfigData), &legacyCfg); err == nil {
					graphCfg := GraphConfig{
						TargetContentLength: legacyCfg.TargetContentLength,
						MinContentLength:    legacyCfg.MinContentLength,
						MaxChunkSize:        legacyCfg.MaxChunkSize,
					}
					if err := asset.SetConfig(graphCfg); err != nil {
						slog.Warn("knowledge migration: failed to set config", "kb_id", kb.ID, "error", err)
					}
				}
			}

			// Force the same ID so FalkorDB graph references (kg_<id>) still work
			// after the admin renames kb_<id> → kg_<id>
			asset.ID = kb.ID
			if err := tx.Create(&asset).Error; err != nil {
				return fmt.Errorf("create asset for kb %d: %w", kb.ID, err)
			}

			// Migrate source associations: find sources with this kb_id (raw SQL since
			// the new KnowledgeSource model no longer has a KbID field)
			var sourceIDs []uint
			if err := tx.Raw(
				"SELECT id FROM ai_knowledge_sources WHERE kb_id = ?", kb.ID,
			).Scan(&sourceIDs).Error; err != nil {
				slog.Warn("knowledge migration: failed to find sources for kb", "kb_id", kb.ID, "error", err)
				continue
			}

			for _, srcID := range sourceIDs {
				link := KnowledgeAssetSource{AssetID: asset.ID, SourceID: srcID}
				if err := tx.Create(&link).Error; err != nil {
					slog.Warn("knowledge migration: failed to link source", "asset_id", asset.ID, "source_id", srcID, "error", err)
				}
			}

			slog.Info("knowledge migration: migrated KB",
				"kb_id", kb.ID, "asset_id", asset.ID, "sources", len(sourceIDs))
		}

		return nil
	})
}

// mapLegacyStatus maps old compile_status values to new asset status values.
func mapLegacyStatus(s string) string {
	switch s {
	case "compiling":
		return AssetStatusBuilding
	case "compiled":
		return AssetStatusReady
	case "error":
		return AssetStatusError
	default:
		return AssetStatusIdle
	}
}
