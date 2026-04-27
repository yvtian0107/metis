package domain

import "errors"

var (
	ErrProductKeyNotFound = errors.New("error.license.product_key_not_found")
	ErrBulkReissueTooMany = errors.New("error.license.bulk_reissue_too_many")
)

type RotateKeyImpact struct {
	AffectedCount  int64 `json:"affectedCount"`
	CurrentVersion int   `json:"currentVersion"`
}
