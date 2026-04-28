# Tab 6 Beds Management - Integration Test Guide

## Overview
This document explains how to run integration tests for Tab 6 (Manajemen Beds) functionality.

## Test Files
- `internal/repository/beds_repository_integration_test.go` - Database integration tests
- `internal/handler/beds_handler_mock_test.go` - Handler unit tests (mock-based)

## What The Tests Cover

### 1. TestGetBedsByRoom_Integration
Tests the core `GetBedsByRoom` function that powers the accordion display in Tab 6:
- Retrieves all distinct class rooms
- For each class room, fetches beds data grouped by kamar
- Logs detailed information about the response structure
- Identifies if no data is returned (potential query issue)

### 2. TestGetBedsByRoom_EmptyClassRoom
Tests behavior with non-existent class room ID:
- Verifies default response structure
- Ensures no errors on empty results

### 3. TestGetBedsByRoom_DataIntegrity
Checks for common data integrity issues that cause accordion problems:
- **Empty/NULL kamar in sk_bed**: Causes grouping key fallback to namaruang
- **Empty/NULL kamar in beds**: Causes beds to be assigned to first available group
- **Mismatched kamar values**: When beds.kamar doesn't match sk_bed.kamar, new groups are created

### 4. TestGetKamarByClassRoom_Integration
Tests the kamar filtering endpoint:
- Verifies kamar list retrieval for each class room

### 5. TestUpsertBeds_BasicOperation
Tests the upsert functionality:
- Verifies INSERT/UPDATE operations work correctly

## How to Run Tests

### Step 1: Set Database Connection
Set the `TEST_DATABASE_DSN` environment variable with your SQL Server connection string:

**Windows (PowerShell):**
```powershell
$env:TEST_DATABASE_DSN="sqlserver://localhost:1433?database=your_db&user id=your_user&password=your_password"
```

**Windows (CMD):**
```cmd
set TEST_DATABASE_DSN=sqlserver://localhost:1433?database=your_db&user id=your_user&password=your_password
```

**Linux/Mac:**
```bash
export TEST_DATABASE_DSN="sqlserver://localhost:1433?database=your_db&user id=your_user&password=your_password"
```

### Step 2: Run All Beds Tests
```bash
go test -v ./internal/repository -run TestGetBedsByRoom
```

### Step 3: Run Specific Test
```bash
# Test only the main GetBedsByRoom function
go test -v ./internal/repository -run TestGetBedsByRoom_Integration

# Test data integrity checks
go test -v ./internal/repository -run TestGetBedsByRoom_DataIntegrity

# Test empty room behavior
go test -v ./internal/repository -run TestGetBedsByRoom_EmptyClassRoom
```

### Step 4: View Detailed Output
```bash
go test -v -run TestGetBedsByRoom_DataIntegrity ./internal/repository 2>&1 | tee test_output.log
```

## Expected Output

### Success Example:
```
=== RUN   TestGetBedsByRoom_Integration
=== RUN   TestGetBedsByRoom_Integration/GetDistinctClassRooms_should_return_active_class_rooms
    beds_repository_integration_test.go:42: Found 5 distinct class rooms
    beds_repository_integration_test.go:44:   [0] class_room_id = "VIP"
    beds_repository_integration_test.go:44:   [1] class_room_id = "KELAS_I"
    ...
=== RUN   TestGetBedsByRoom_Integration/GetBedsByRoom_for_each_class_room
=== RUN   TestGetBedsByRoom_Integration/GetBedsByRoom_for_each_class_room/class_room_id=VIP
    beds_repository_integration_test.go:64: Mode: edit
    beds_repository_integration_test.go:65: Number of kamar groups: 3
    beds_repository_integration_test.go:68:   [0] Kamar = "VIP 1"
    beds_repository_integration_test.go:69:       Defaults: id_tt_siranap="TT001", covid="0", id_kelas="1", nm_kelas="VIP"
    beds_repository_integration_test.go:75:       Rows count: 5
    ...
--- PASS: TestGetBedsByRoom_Integration (0.52s)
```

### Common Issues Indicated by Test Results:

#### Issue 1: No Data in Accordion
If test shows:
```
WARNING: No kamar groups returned for class_room_id="VIP"
```
**Possible causes:**
1. No active records in `sk_bed` (all have `tgl_berakhir` set)
2. Mismatch between `sk_bed.kamar` and `beds.kamar` values
3. Empty/NULL `kamar` values causing grouping issues

**Solution:** Check data integrity test output for specific mismatches.

#### Issue 2: Mismatched Kamar Values
If test shows:
```
MISMATCH: class_room_id="VIP", beds.kamar="Kamar A" not found in sk_bed
```
**Cause:** The beds table has kamar values that don't exist in sk_bed.

**Solution:** Ensure `sk_bed` has active records for all kamar values used in `beds`.

#### Issue 3: Empty Kamar Values
If test shows:
```
WARNING: class_room_id="VIP" has 10 beds with empty kamar
```
**Cause:** Beds records have empty `kamar` field.

**Solution:** The code has fallback logic to assign these to first group, but if multiple groups exist, misgrouping may occur.

## Troubleshooting

### Test Skipped
```
beds_repository_integration_test.go:XX: TEST_DATABASE_DSN not set, skipping integration test
```
**Solution:** Set the `TEST_DATABASE_DSN` environment variable (see Step 1 above).

### Database Connection Failed
```
Failed to ping database: unable to open tcp connection
```
**Solution:** Verify DSN string format and database server is running.

### No Class Rooms Found
```
WARNING: No active class rooms found in sk_bed table
```
**Solution:** Check that `sk_bed` table has records with `tgl_berakhir IS NULL`.

## Understanding The Query Logic

The `GetBedsByRoom` function uses a **two-phase query approach**:

### Phase 1: Fetch Defaults from `sk_bed`
```sql
SELECT 
    ISNULL(NULLIF(LTRIM(RTRIM(kamar)), ''), ISNULL(namaruang, '')) as kamar_key,
    id_tt_siranap, covid, kodekelas, namakelas
FROM sk_bed WITH (NOLOCK)
WHERE class_room_id = @p1 AND tgl_berakhir IS NULL
```

**Key Logic:**
- `kamar_key` = trimmed `kamar`, fallback to `namaruang` if empty/NULL
- This `kamar_key` becomes the **accordion section name**

### Phase 2: Fetch Beds from `beds` Table
```sql
SELECT bed_id, ISNULL(kamar, ''), room_id, ...
FROM beds WITH (NOLOCK)
WHERE class_room_id = @p1
ORDER BY kamar, bed_id
```

**Grouping Logic:**
- Beds are grouped by their `kamar` field
- If `kamar` is empty AND only one sk_bed group exists → assign to that group
- If `kamar` doesn't match any sk_bed group → create new group

### Common Problem: Data Not Showing in Accordion

**Scenario:** User searches by class_room_id, clicks "Tampilkan Data", but accordion is empty.

**Root Causes Identified by Tests:**

1. **No Active SK Records**
   - Query: `WHERE class_room_id = @p1 AND tgl_berakhir IS NULL`
   - Problem: All records for that class_room_id have `tgl_berakhir` set
   - Fix: Ensure there are active SK records with `tgl_berakhir IS NULL`

2. **Kamar Mismatch**
   - sk_bed has: `kamar = "VIP A"`
   - beds has: `kamar = "VIP"` (different value)
   - Result: New accordion group created with empty defaults
   - Fix: Synchronize kamar values between tables

3. **Empty Kamar in Both Tables**
   - sk_bed: `kamar = ""`, `namaruang = "Ruangan VIP"`
   - beds: `kamar = ""`
   - Result: Grouping key becomes "Ruangan VIP" from sk_bed, but beds key is ""
   - Fallback assigns to first group, but may cause issues with multiple groups

4. **Wrong class_room_id Parameter**
   - User selects wrong value from dropdown
   - No data exists for that class_room_id
   - Fix: Verify class_room_id exists in database

## Next Steps After Running Tests

1. Run the data integrity test first:
   ```bash
   go test -v ./internal/repository -run TestGetBedsByRoom_DataIntegrity
   ```

2. Review the output for any WARNING or MISMATCH messages

3. If issues are found, check the specific class_room_id that's problematic:
   ```bash
   go test -v ./internal/repository -run "TestGetBedsByRoom_Integration/GetBedsByRoom_for_each_class_room/class_room_id=YOUR_ROOM_ID"
   ```

4. Use the detailed logging to identify which phase of the query is failing

5. Fix the underlying data issue in the database or adjust the query logic
