## 1. Test Infrastructure & Refactoring for Testability

- [x] 1.1 Create `internal/app/license/testutil_test.go` with `setupTestDB(t)` helper that spins up an in-memory SQLite DB and auto-migrates all license models
- [x] 1.2 Refactor `deriveLifecycleStatus` to accept an explicit `now time.Time` parameter; update all call sites to pass `time.Now()`
- [x] 1.3 Introduce a package-level `timeNow = time.Now` fallback (or clock struct) for any additional time-bound helpers that cannot change signatures

## 2. Crypto & Validation Pure-Function Tests (TDD)

- [x] 2.1 Write red tests for `Canonicalize` covering empty object, nested map key sorting, arrays, and mixed types; implement until green
- [x] 2.2 Add fuzz test `FuzzCanonicalizeDeterminism` ensuring canonicalized output is stable across repeated calls
- [x] 2.3 Write red tests for `EncryptLicenseFile`/`DecryptLicenseFile` round-trip and invalid format handling; implement until green
- [x] 2.4 Add fuzz test `FuzzEncryptDecryptRoundTrip` with random plaintext, registration code, and product name
- [x] 2.5 Write red tests for `validateConstraintSchema` covering valid schema, duplicate module keys, duplicate feature keys, and invalid feature types; implement until green
- [x] 2.6 Write red tests for `validateConstraintValues` covering missing module keys, out-of-range numbers, invalid enum values, and multi-select type mismatches; implement until green
- [x] 2.7 Add fuzz test `FuzzValidateConstraintSchemaNoPanic` feeding random JSON bytes into the validator

## 3. Product & Plan Service Integration Tests (TDD)

- [x] 3.1 Write red test for `ProductService.CreateProduct` asserting product creation and auto-generated initial `ProductKey`; construct service directly with in-memory DB; implement until green
- [x] 3.2 Write red tests for `ProductService.UpdateStatus` covering all allowed transitions and blocking illegal ones; implement until green
- [x] 3.3 Write red test for `ProductService.RotateKey` asserting old key revocation and new key version increment; implement until green
- [x] 3.4 Write red tests for `PlanService.CreatePlan` and `UpdatePlan` with constraint value validation against product schema; implement until green
- [x] 3.5 Write red test for `PlanService.SetDefaultPlan` asserting uniqueness of default plan per product; implement until green

## 4. License Lifecycle Service Integration Tests (TDD)

- [x] 4.1 Write red happy-path test for `LicenseService.IssueLicense` with published product, active licensee, and valid registration code; implement until green
- [x] 4.2 Write red guard-clause tests for `IssueLicense` covering unpublished product, inactive licensee, missing key, already-bound registration code, and expired registration code
- [x] 4.3 Write red test for `LicenseService.UpgradeLicense` asserting original license revocation, registration code rebinding, and `original_license_id` linkage; implement until green
- [x] 4.4 Write red tests for `SuspendLicense`, `ReactivateLicense`, `RevokeLicense`, and `RenewLicense` covering state machine guards and timestamp updates
- [x] 4.5 Write red test for `LicenseService.BulkReissueLicenses` asserting successful reissue of outdated licenses and the 100-item limit guard
- [x] 4.6 Write red test for `LicenseService.ExportLicFile` covering happy path and rejection of revoked licenses

## 5. Licensee Service & Handler Tests

- [x] 5.1 Write red tests for `LicenseeService.Create`/`Update`/`UpdateStatus` using in-memory DB; implement until green
- [x] 5.2 Write handler-level tests for `ProductHandler.Get` (404 on missing ID) and `ProductHandler.UpdateSchema` (400 on invalid constraint JSON) using `gin.CreateTestContext` with stubbed service
- [x] 5.3 Write handler-level test for `ProductHandler.BulkReissue` (400 on too many IDs) using stubbed service

## 6. Final Verification & Cleanup

- [x] 6.1 Run `go test ./internal/app/license/...` and ensure all tests pass locally
- [x] 6.2 Run `go test -fuzz=30s` on each fuzz target and verify no crashes
- [x] 6.3 Verify `go build -tags dev ./cmd/server/` still compiles cleanly after any refactor
