-- Quick Diagnostic Query for Tab 6 Beds Management Issues
-- Run this in SQL Server Management Studio or Azure Data Studio
-- Replace 'YOUR_DATABASE' with your actual database name

USE YOUR_DATABASE;
GO

-- ========================================
-- 1. Check Active Class Rooms
-- ========================================
PRINT '=== Active Class Rooms (sk_bed with tgl_berakhir IS NULL) ===';
SELECT 
    class_room_id,
    COUNT(*) as total_records,
    COUNT(DISTINCT kamar) as distinct_kamar,
    COUNT(DISTINCT namaruang) as distinct_namaruang
FROM sk_bed WITH (NOLOCK)
WHERE tgl_berakhir IS NULL
GROUP BY class_room_id
ORDER BY class_room_id;
GO

-- ========================================
-- 2. Check for Empty/NULL Kamar Issues
-- ========================================
PRINT '=== sk_bed Records with Empty/NULL Kamar ===';
SELECT 
    class_room_id,
    kamar,
    namaruang,
    COUNT(*) as record_count,
    CASE 
        WHEN LTRIM(RTRIM(ISNULL(kamar, ''))) = '' THEN 'EMPTY_KAMAR'
        ELSE 'HAS_KAMAR'
    END as kamar_status
FROM sk_bed WITH (NOLOCK)
WHERE tgl_berakhir IS NULL
GROUP BY class_room_id, kamar, namaruang
ORDER BY class_room_id, kamar_status DESC;
GO

-- ========================================
-- 3. Check beds Table Data
-- ========================================
PRINT '=== beds Table Summary by class_room_id ===';
SELECT 
    class_room_id,
    COUNT(*) as total_beds,
    COUNT(DISTINCT kamar) as distinct_kamar,
    SUM(CASE WHEN ISNULL(kamar, '') = '' THEN 1 ELSE 0 END) as empty_kamar_count
FROM beds WITH (NOLOCK)
GROUP BY class_room_id
ORDER BY class_room_id;
GO

-- ========================================
-- 4. Find Specific Problem: No Data for Specific class_room_id
-- ========================================
-- Replace 'YOUR_CLASS_ROOM_ID' with the actual value you're testing
DECLARE @TestClassRoomID VARCHAR(100) = 'YOUR_CLASS_ROOM_ID';

PRINT '=== Testing Specific class_room_id: ' + @TestClassRoomID + ' ===';

-- Check sk_bed
SELECT 
    'sk_bed' as source_table,
    class_room_id,
    kamar,
    namaruang,
    id_tt_siranap,
    kodekelas,
    namakelas,
    tgl_berlaku,
    tgl_berakhir
FROM sk_bed WITH (NOLOCK)
WHERE class_room_id = @TestClassRoomID
ORDER BY tgl_berakhir DESC, kamar;

-- Check beds
SELECT 
    'beds' as source_table,
    class_room_id,
    kamar,
    room_id,
    id_kelas,
    nm_kelas,
    COUNT(*) as bed_count
FROM beds WITH (NOLOCK)
WHERE class_room_id = @TestClassRoomID
GROUP BY class_room_id, kamar, room_id, id_kelas, nm_kelas
ORDER BY kamar;
GO

-- ========================================
-- 5. Simulate the Actual Query Logic
-- ========================================
PRINT '=== Simulate GetBedsByRoom Query Logic ===';

-- Replace with your class_room_id
DECLARE @TestRoomID VARCHAR(100) = 'YOUR_CLASS_ROOM_ID';

-- Phase 1: sk_bed defaults (this creates kamar groups)
SELECT 
    'Phase 1: sk_bed defaults' as phase,
    ISNULL(NULLIF(LTRIM(RTRIM(kamar)), ''), ISNULL(namaruang, '')) as kamar_key,
    id_tt_siranap,
    covid,
    ISNULL(kodekelas, '') as id_kelas,
    ISNULL(namakelas, '') as nm_kelas
FROM sk_bed WITH (NOLOCK)
WHERE class_room_id = @TestRoomID AND tgl_berakhir IS NULL
ORDER BY kamar_key;

-- Phase 2: beds data (this populates the groups)
SELECT 
    'Phase 2: beds data' as phase,
    bed_id,
    ISNULL(kamar, '') as kamar,
    room_id,
    id_kelas,
    nm_kelas
FROM beds WITH (NOLOCK)
WHERE class_room_id = @TestRoomID
ORDER BY kamar, bed_id;
GO

-- ========================================
-- 6. Identify Mismatches
-- ========================================
PRINT '=== Kamar Value Mismatches Between sk_bed and beds ===';

WITH sk_kamars AS (
    SELECT DISTINCT 
        class_room_id,
        ISNULL(NULLIF(LTRIM(RTRIM(kamar)), ''), ISNULL(namaruang, '')) as kamar_key
    FROM sk_bed WITH (NOLOCK)
    WHERE tgl_berakhir IS NULL
),
beds_kamars AS (
    SELECT DISTINCT 
        class_room_id,
        ISNULL(kamar, '') as kamar_val
    FROM beds WITH (NOLOCK)
)
SELECT 
    b.class_room_id,
    b.kamar_val as beds_kamar,
    s.kamar_key as sk_bed_kamar,
    CASE 
        WHEN s.kamar_key IS NULL THEN 'MISMATCH - Will create new group'
        ELSE 'MATCH'
    END as status
FROM beds_kamars b
LEFT JOIN sk_kamars s ON b.class_room_id = s.class_room_id AND b.kamar_val = s.kamar_key
WHERE s.kamar_key IS NULL
ORDER BY b.class_room_id, b.kamar_val;
GO
