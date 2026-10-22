package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/model"
	"strings"

	"github.com/RoaringBitmap/roaring/v2"
)

func (e *BuildEvaluator) bitmapCount(ctx context.Context, buildID string, in model.RequestInput) (int64, bool, error) {
	var required []string
	if err := e.db.Pool.QueryRow(ctx, `SELECT required_outputs FROM segment_build_v0 WHERE build_id=$1 AND status='READY'`,
		buildID).Scan(&required); err != nil {
		return 0, false, err
	}
	found := false
	for _, item := range required {
		if item == "bitmaps" {
			found = true
			break
		}
	}
	if !found {
		return 0, false, nil
	}
	if len(in.Segments) == 0 {
		return 0, false, nil
	}
	var result *roaring.Bitmap
	if in.Base != "" && in.Base != "all_members" {
		base, supported, err := e.segmentBitmap(ctx, buildID, in.Base)
		if err != nil || !supported {
			return 0, supported, err
		}
		result = base
	}
	for i, selection := range in.Segments {
		if len(selection.Filters) != 0 {
			return 0, false, nil
		}
		bitmap, supported, err := e.segmentBitmap(ctx, buildID, selection.ID)
		if err != nil || !supported {
			return 0, supported, err
		}
		if result == nil {
			result = bitmap
			continue
		}
		if i > 0 && strings.EqualFold(in.Segments[i-1].Operation, "OR") {
			result.Or(bitmap)
		} else {
			result.And(bitmap)
		}
	}
	consent, err := e.loadBitmap(ctx, buildID, "consent", "yes", 1)
	if err != nil {
		return 0, true, err
	}
	result.And(consent)
	return int64(result.GetCardinality()), true, nil
}

func (e *BuildEvaluator) segmentBitmap(ctx context.Context, buildID, id string) (*roaring.Bitmap, bool, error) {
	var kind, status string
	var version int
	err := e.db.Pool.QueryRow(ctx, `SELECT kind,definition_status,definition_version
		FROM segments_v0 WHERE id=$1 AND is_visible`, id).Scan(&kind, &status, &version)
	if err != nil {
		return nil, false, err
	}
	if status != "approved" || (kind != "option" && kind != "predefined") {
		return nil, false, nil
	}
	bitmap, err := e.loadBitmap(ctx, buildID, kind, id, version)
	return bitmap, true, err
}

func (e *BuildEvaluator) loadBitmap(ctx context.Context, buildID, kind, id string, version int) (*roaring.Bitmap, error) {
	var storedVersion, encodingVersion int
	var cardinality int64
	var checksum string
	var payload []byte
	err := e.db.Pool.QueryRow(ctx, `SELECT definition_version,encoding_version,cardinality,checksum,payload
		FROM member_bitmap_v0 WHERE build_id=$1 AND predicate_kind=$2 AND predicate_id=$3`,
		buildID, kind, id).Scan(&storedVersion, &encodingVersion, &cardinality, &checksum, &payload)
	if err != nil {
		return nil, err
	}
	if storedVersion != version {
		return nil, fmt.Errorf("bitmap definition version mismatch for %s/%s", kind, id)
	}
	if encodingVersion != 1 {
		return nil, fmt.Errorf("unsupported bitmap encoding version %d for %s/%s", encodingVersion, kind, id)
	}
	digest := sha256.Sum256(payload)
	if hex.EncodeToString(digest[:]) != checksum {
		return nil, fmt.Errorf("bitmap checksum mismatch for %s/%s", kind, id)
	}
	bitmap := roaring.New()
	if err := bitmap.UnmarshalBinary(payload); err != nil {
		return nil, err
	}
	if int64(bitmap.GetCardinality()) != cardinality {
		return nil, fmt.Errorf("bitmap cardinality mismatch for %s/%s", kind, id)
	}
	return bitmap, nil
}
