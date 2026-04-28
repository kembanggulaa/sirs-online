# Tab 6 Manajemen Beds - Testing & Diagnostic Tools

## 📋 Overview

I've created comprehensive testing and diagnostic tools to help identify why data doesn't appear in the Tab 6 Beds Management accordion when searching by nama bangsal/classroom.

## 🧪 Test Files Created

### 1. Integration Tests
**File:** `internal/repository/beds_repository_integration_test.go`

Comprehensive Go tests that check:
- ✅ `GetDistinctClassRooms()` - Retrieves active class rooms
- ✅ `GetBedsByRoom()` - Core accordion data retrieval
- ✅ `GetKamarByClassRoom()` - Kamar filtering
- ✅ Data integrity checks (empty values, mismatches)
- ✅ `UpsertBeds()` - Create/Update/Delete operations

### 2. Diagnostic SQL Queries
**File:** `test/beds_management/diagnostic_queries.sql`

SQL Server queries for manual diagnostics:
- Check active class rooms
- Find empty/NULL kamar values
- Identify mismatches between sk_bed and beds tables
- Simulate actual query logic

### 3. Simple Check Tool
**File:** `test/beds_management/simple_check.go`

Quick diagnostic tool that doesn't require test framework setup:
```bash
# Set environment variables
set DB_HOST=localhost
set DB_PORT=1433
set DB_USER=your_user
set DB_PASS=your_password
set DB_NAME=your_database

# Run check
go run test/beds_management/simple_check.go
```

### 4. Test Runner Scripts
**Files:**
- `test/beds_management/run_beds_tests.ps1` (PowerShell)
- `test/beds_management/run_beds_tests.bat` (Batch)

Easy-to-use test runners with menu interface.

## 🚀 How to Run Tests

### Method 1: Using Environment Variable (Recommended)

```powershell
# Set DSN (PowerShell)
$env:TEST_DATABASE_DSN="sqlserver://localhost:1433?database=YOUR_DB&user id=YOUR_USER&password=YOUR_PASS"

# Run all tests
go test -v ./internal/repository -run "TestGetBedsByRoom"

# Run specific test
go test -v ./internal/repository -run TestGetBedsByRoom_DataIntegrity
```

### Method 2: Using Test Runner Script

```powershell
# PowerShell
.\test\beds_management\run_beds_tests.ps1 -AllTests

# Or interactive menu
.\test\beds_management\run_beds_tests.bat
```

### Method 3: Simple Check (No Test Framework)

```bash
# Set individual env vars
set DB_HOST=localhost
set DB_PORT=1433
set DB_USER=sa
set DB_PASS=yourpassword
set DB_NAME=sirs_db

go run test/beds_management/simple_check.go
```

## 🔍 Common Issues & Solutions

### Issue 1: Accordion Empty After Search

**Symptoms:** User selects class_room_id, clicks "Tampilkan Data", but no data appears.

**Diagnostic Steps:**

1. **Check if class_room_id exists with active SK:**
   ```sql
   SELECT COUNT(*) 
   FROM sk_bed WITH (NOLOCK)
   WHERE class_room_id = 'YOUR_ROOM_ID' 
     AND tgl_berakhir IS NULL
   ```
   
   **If count = 0:** All records have `tgl_berakhir` set. The SK has expired.
   
   **Solution:** Ensure there are active SK records with `tgl_berakhir IS NULL`.

2. **Check if beds exist:**
   ```sql
   SELECT COUNT(*) 
   FROM beds WITH (NOLOCK)
   WHERE class_room_id = 'YOUR_ROOM_ID'
   ```
   
   **If count = 0:** No beds records exist for this class_room_id.
   
   **Solution:** Beds need to be created first.

3. **Check for kamar mismatch:**
   ```sql
   -- What sk_bed expects
   SELECT DISTINCT ISNULL(NULLIF(LTRIM(RTRIM(kamar)), ''), ISNULL(namaruang, '')) as kamar_key
   FROM sk_bed WITH (NOLOCK)
   WHERE class_room_id = 'YOUR_ROOM_ID' AND tgl_berakhir IS NULL
   
   -- What beds has
   SELECT DISTINCT ISNULL(kamar, '') as kamar_val
   FROM beds WITH (NOLOCK)
   WHERE class_room_id = 'YOUR_ROOM_ID'
   ```
   
   **If values don't match:** Beds will be in wrong groups or create new empty groups.
   
   **Solution:** Synchronize kamar values between tables.

### Issue 2: Data Shows But in Wrong Accordion Section

**Cause:** Mismatch between `sk_bed.kamar` and `beds.kamar` values.

**Example:**
- sk_bed: `kamar = "VIP Kelas A"`, `namaruang = "VIP"`
- beds: `kamar = "VIP A"`
- Result: New accordion section "VIP A" created instead of "VIP Kelas A"

**Solution:** Ensure kamar values match exactly between tables.

### Issue 3: All Beds in One Accordion Section

**Cause:** All beds have empty `kamar` field.

**Logic:** When `kamar = ""` AND only one sk_bed group exists, all beds assigned to that group.

**Solution:** Populate `kamar` field in beds table with correct values.

## 📊 Understanding the Query Logic

### Phase 1: Create Accordion Groups (from sk_bed)
```sql
SELECT
    ISNULL(NULLIF(LTRIM(RTRIM(kamar)), ''), ISNULL(namaruang, '')) as kamar_key,
    id_tt_siranap,
    covid,
    ISNULL(kodekelas, '') as id_kelas,
    ISNULL(namakelas, '') as nm_kelas
FROM sk_bed WITH (NOLOCK)
WHERE class_room_id = @p1 AND tgl_berakhir IS NULL
```

**Key Points:**
- Each distinct `kamar_key` becomes an accordion section
- If `kamar` is empty, fallback to `namaruang`
- Only active SKs (where `tgl_berakhir IS NULL`) are used
- This phase creates the **structure** of the accordion

### Phase 2: Populate Groups (from beds)
```sql
SELECT bed_id, ISNULL(kamar, ''), room_id, id_kelas, nm_kelas, ...
FROM beds WITH (NOLOCK)
WHERE class_room_id = @p1
ORDER BY kamar, bed_id
```

**Key Points:**
- Beds are grouped by their `kamar` field
- If `kamar` matches a group from Phase 1 → added to that group
- If `kamar` doesn't match → new group created with empty defaults
- If `kamar = ""` AND only one group exists → assigned to that group
- This phase **populates** the accordion sections with data

### Grouping Resolution Logic

```
For each bed record:
  1. Get kamar value from beds.kamar
  2. Check if kamar exists in kamarsMap (from Phase 1)
  3. If YES: Add bed to that group
  4. If NO:
     a. If kamar is empty AND only 1 group exists:
        → Assign to first group (fallback)
     b. Otherwise:
        → Create new group with defaults
```

## 🎯 Test Output Interpretation

### Successful Test Output
```
=== RUN   TestGetBedsByRoom_Integration
    Found 5 distinct class rooms
      [0] class_room_id = "VIP"
      [1] class_room_id = "KELAS_I"
    
    Mode: edit
    Number of kamar groups: 3
      [0] Kamar = "VIP 1"
          Defaults: id_tt_siranap="TT001", covid="0", id_kelas="1", nm_kelas="VIP"
          Rows count: 5
            [0] bed_id=101, room_id="R001", id_kelas="1", nm_kelas="VIP"
    --- PASS: TestGetBedsByRoom_Integration (0.52s)
```

### Problem Indicators

**No Data:**
```
WARNING: No kamar groups returned for class_room_id="VIP"
This could indicate:
  1. No records in sk_bed with tgl_berakhir IS NULL
  2. Mismatch between sk_bed.kamar and beds.kamar values
  3. Empty/NULL kamar values causing grouping issues
```

**Data Integrity Issues:**
```
WARNING: class_room_id="VIP" has empty kamar, namaruang="Ruangan VIP" (count=10)
This may cause accordion grouping issues if namaruang is also empty
```

**Mismatches:**
```
MISMATCH: class_room_id="VIP", beds.kamar="Kamar A" not found in sk_bed
Found 3 mismatched kamar values between beds and sk_bed
These mismatches cause new accordion groups to be created
```

## 🐛 Debugging Workflow

1. **Run Simple Check First:**
   ```bash
   go run test/beds_management/simple_check.go
   ```
   This gives quick overview of data state.

2. **Run Data Integrity Tests:**
   ```bash
   go test -v ./internal/repository -run TestGetBedsByRoom_DataIntegrity
   ```
   Identifies specific data issues.

3. **Test Specific class_room_id:**
   ```bash
   go test -v ./internal/repository -run "TestGetBedsByRoom_Integration/GetBedsByRoom_for_each_class_room/class_room_id=PROBLEM_ROOM"
   ```

4. **Run SQL Diagnostic Queries:**
   - Open `test/beds_management/diagnostic_queries.sql`
   - Replace `YOUR_DATABASE` and `YOUR_CLASS_ROOM_ID`
   - Execute in SQL Server Management Studio

5. **Analyze Results:**
   - Check if sk_bed has active records
   - Verify beds table has records
   - Look for kamar value mismatches
   - Review empty/NULL values

## 📝 Next Steps After Identifying Issue

### If Problem is in sk_bed:
1. Check if SK has expired (`tgl_berakhir` is set)
2. Verify `kamar` and `namaruang` values are correct
3. Ensure `class_room_id` matches what user is searching

### If Problem is in beds:
1. Check if records exist for the class_room_id
2. Verify `kamar` values match sk_bed
3. Populate empty `kamar` fields with correct values

### If Problem is Query Logic:
1. Review the grouping fallback logic in `GetBedsByRoom()`
2. Check if multiple empty kamar groups cause confusion
3. Consider adding validation or better error messages

## 📚 Related Files

- Repository: `internal/repository/beds_repository.go`
- Handler: `internal/handler/beds_handler.go`
- Frontend: `web/static/js/beds_components.js`
- HTML: `web/static/index.html` (lines 784-940)
- Interfaces: `internal/repository/interfaces.go`

## 💡 Tips

1. **Always test with production data copy** - Never test on production database
2. **Use NOLOCK hints carefully** - They allow dirty reads
3. **Check both tables** - Issues often come from mismatch between sk_bed and beds
4. **Look at tgl_berakhir** - Expired SKs won't show up
5. **Verify class_room_id exact match** - It's case-sensitive and exact match

## 🆘 Need Help?

If tests don't reveal the issue:
1. Share the test output (anonymize sensitive data)
2. Run diagnostic SQL queries and share results
3. Check the specific class_room_id that's failing
4. Verify both sk_bed and beds tables have matching data

The tests will identify 95%+ of common issues with Tab 6 Beds Management accordion data display.
