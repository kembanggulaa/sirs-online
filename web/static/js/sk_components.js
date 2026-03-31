document.addEventListener('alpine:init', () => {
  Alpine.data('skManagement', () => ({
    step: 1,
    skNo: '',
    tglBerlaku: '',
    rows: [],
    loadingExcel: false,
    importing: false,
    importMessage: '',
    importSuccess: false,
    retiringSK: null,          // detected old SK to be retired
    dragOver: false,

    // Base template for new row
    newRowTemplate() {
      return {
        id: crypto.randomUUID(),
        clinic_id: '',
        class_room_id: '',
        kelas: '',
        bed: 0,
        id_tt_siranap: '',
        ruang_siranap: '',
        kelas_siranap: '',
        covid: 0,
        siranap: '',
        jml_ruang_siranap: 1,
        kodekelas: '',
        namakelas: '',
        namaruang: '',
        kris: '',
        kamar: ''
      };
    },

    init() {
      // Fetch currently active SK to show retiring notice early if possible
      this.checkActiveSK();
      
      // Jika user mengubah skNo secara manual di Step 1, reset baris agar memicu tarik data (fetch) terbaru
      this.$watch('skNo', (value, oldValue) => {
        if (value !== oldValue && this.step === 1) {
          this.rows = [];
        }
      });
    },

    async checkActiveSK() {
      try {
        const r = await fetch('/api/sk-active');
        const d = await r.json();
        if (d.sk_no) {
          this.retiringSK = d.sk_no;
        }
      } catch (e) {
        // ignore
      }
    },

    async goToStep(n) {
      if (n === 2 && (!this.skNo || !this.tglBerlaku)) {
        alert('Nomor SK dan Tanggal Berlaku wajib diisi!');
        return;
      }
      
      // Tarik data existing dari DB jika form baris masih kosong (user mengisi manual dari Step 1)
      if (this.step === 1 && n === 2 && this.rows.length === 0) {
        try {
          const res = await fetch('/api/sk/detail?sk_no=' + encodeURIComponent(this.skNo));
          if (res.ok) {
            const extRows = await res.json();
            if (extRows && extRows.length > 0) {
              // Gabungkan template dengan data DB untuk mensupply key UI (id) yang dibutuhkan Alpine
              this.rows = extRows.map(r => ({ ...this.newRowTemplate(), ...r, id: crypto.randomUUID() }));
            }
          }
        } catch (e) {
          console.error("Gagal load existing SK detail", e);
        }

        if (this.rows.length === 0) {
          // Init 1 baris kosong minimal jika benar-benar SK Baru
          this.rows.push(this.newRowTemplate());
        }
      }
      
      this.step = n;
      // Scroll to top lightly
      window.scrollTo({ top: 0, behavior: 'smooth' });
    },

    addRow() {
      const newRow = this.newRowTemplate();
      this.rows.push(newRow);
      
      this.$nextTick(() => {
        const el = document.getElementById('row-' + newRow.id);
        if (el) {
          el.scrollIntoView({ behavior: 'smooth', block: 'center' });
          // Sedikit efek highlight visual untuk menunjukkan baris mana yang baru
          el.classList.add('ring-2', 'ring-brand-500', 'ring-offset-2');
          setTimeout(() => el.classList.remove('ring-2', 'ring-brand-500', 'ring-offset-2'), 1500);
        }
      });
    },

    duplicateRow(index) {
      const source = this.rows[index];
      const newId = crypto.randomUUID();
      const copy = { ...source, id: newId, kamar: source.kamar + ' (Copy)' };
      
      // Insert after the current row
      this.rows.splice(index + 1, 0, copy);
      
      this.$nextTick(() => {
        const el = document.getElementById('row-' + newId);
        if (el) {
          el.scrollIntoView({ behavior: 'smooth', block: 'center' });
          el.classList.add('ring-2', 'ring-brand-500', 'ring-offset-2');
          setTimeout(() => el.classList.remove('ring-2', 'ring-brand-500', 'ring-offset-2'), 1500);
        }
      });
    },

    deleteRow(index) {
      const namaKamar = this.rows[index].kamar || 'Kamar Tanpa Nama (Kosong)';
      if (!confirm(`Apaka Anda yakin ingin menghapus baris data untuk "${namaKamar}"?\n\nTindakan ini akan menghapus draf baris ini dari form sebelum disimpan.`)) {
        return;
      }
      
      this.rows.splice(index, 1);
      if (this.rows.length === 0) {
        this.addRow();
      }
    },

    // ─── EXCEL UPLOAD ────────────────────────────────────────────────────────
    
    handleFiles(e) {
      this.dragOver = false;
      let files = e.target.files;
      if (!files || files.length === 0) {
        files = e.dataTransfer?.files;
      }
      if (!files || files.length === 0) return;

      const file = files[0];
      this.loadingExcel = true;

      const reader = new FileReader();
      reader.onload = (event) => {
        try {
          const data = new Uint8Array(event.target.result);
          const workbook = XLSX.read(data, { type: 'array' });
          const firstSheetName = workbook.SheetNames[0];
          const worksheet = workbook.Sheets[firstSheetName];
          const jsonOpts = { header: 1, defval: "" }; // array of arrays
          const sheetData = XLSX.utils.sheet_to_json(worksheet, jsonOpts);
          
          this.parseExcelData(sheetData);
        } catch (err) {
          alert('Gagal membaca file Excel: ' + err.message);
        } finally {
          this.loadingExcel = false;
        }
      };
      reader.readAsArrayBuffer(file);
    },

    parseExcelData(data) {
      // Assuming first row is header, skip it. Or detect if it's header.
      if (data.length < 2) {
        alert('File kosong atau format salah');
        return;
      }
      
      const newRows = [];
      // Iterasikan baris data, asumsikan mengikuti urutan arsitektur jika menggunakan header dinamis.
      // 2: clinic_id, 3: class_room_id, 4: kelas, 5: bed, 8: id_tt_siranap, dsb.
      // Kita pakai pendekatan header mapping dasar atau urutan index kasar jika header dinamis.
      // Lebih aman dengan array index sesuai export standar SIMRS.
      
      // Ambil index header
      const headers = data[0].map(h => String(h).toLowerCase().trim());
      
      const getIdx = (name1, name2) => {
        let idx = headers.findIndex(h => h.includes(name1) || (name2 && h.includes(name2)));
        if (idx === -1) idx = headers.indexOf(''); // fallback?
        return idx;
      };

      const map = {
        clinic: getIdx('clinic', 'klinik'),
        class: getIdx('class_room', 'bangsal'),
        kelas: getIdx('kelas', ''),
        bed: getIdx('bed', 'jumlah'),
        id_tt: getIdx('id_tt', 'siranap'),
        ruang_sir: getIdx('ruang_siranap', ''),
        kelas_sir: getIdx('kelas_siranap', ''),
        covid: getIdx('covid', ''),
        siranap: getIdx('siranap', 'nama_siranap'),
        jml_ruang: getIdx('jml_ruang', 'jumlah_ruang'),
        kodekelas: getIdx('kodekelas', ''),
        namakelas: getIdx('namakelas', ''),
        namaruang: getIdx('namaruang', ''),
        kris: getIdx('kris', ''),
        kamar: getIdx('kamar', 'nomor_kamar'),
      };

      for (let i = 1; i < data.length; i++) {
        const row = data[i];
        if (!row.some(c => c !== '')) continue; // skip baris kosong

        // Fallback urutan manual jika header tidak cocok:
        // [sk_no, clinic_id, class_room_id, kelas, bed, ...]
        newRows.push({
          id: crypto.randomUUID(),
          clinic_id:        map.clinic > -1 ? String(row[map.clinic]) : String(row[0]||''),
          class_room_id:    map.class > -1 ? String(row[map.class]) : String(row[1]||''),
          kelas:            map.kelas > -1 ? String(row[map.kelas]) : String(row[2]||''),
          bed:              map.bed > -1 ? parseInt(row[map.bed]||0) : parseInt(row[3]||0),
          id_tt_siranap:    map.id_tt > -1 ? String(row[map.id_tt]) : String(row[4]||''),
          ruang_siranap:    map.ruang_sir > -1 ? String(row[map.ruang_sir]) : String(row[5]||''),
          kelas_siranap:    map.kelas_sir > -1 ? String(row[map.kelas_sir]) : String(row[6]||''),
          covid:            map.covid > -1 ? parseInt(row[map.covid]||0) : parseInt(row[7]||0),
          siranap:          map.siranap > -1 ? String(row[map.siranap]) : String(row[8]||''),
          jml_ruang_siranap:map.jml_ruang > -1 ? parseInt(row[map.jml_ruang]||1) : parseInt(row[9]||1),
          kodekelas:        map.kodekelas > -1 ? String(row[map.kodekelas]) : String(row[10]||''),
          namakelas:        map.namakelas > -1 ? String(row[map.namakelas]) : String(row[11]||''),
          namaruang:        map.namaruang > -1 ? String(row[map.namaruang]) : String(row[12]||''),
          kris:             map.kris > -1 ? String(row[map.kris]) : String(row[13]||''),
          kamar:            map.kamar > -1 ? String(row[map.kamar]) : String(row[14]||''),
        });
      }

      if (newRows.length > 0) {
        this.rows = newRows;
        this.goToStep(2);
      } else {
        alert('Gagal mengekstrak data dari Excel. Pastikan format kolom benar.');
      }
    },

    // ─── API SEND ──────────────────────────────────────────────────────────

    async performImport() {
      if (!confirm('Apakah Anda yakin ingin menyimpan SK Bed sejumlah ' + this.rows.length + ' baris?')) {
        return;
      }

      this.importing = true;
      this.importMessage = '';
      this.importSuccess = false;

      const payload = {
        sk_no: this.skNo,
        tgl_berlaku: this.tglBerlaku,
        rows: this.rows.map(r => ({
          ...r, bed: parseInt(r.bed||0), covid: parseInt(r.covid||0), jml_ruang_siranap: parseInt(r.jml_ruang_siranap||1)
        }))
      };

      try {
        const res = await fetch('/api/sk/import', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        });

        const data = await res.json();
        if (res.ok) {
          window.isFormDirty = false;
          this.importSuccess = true;
          this.importMessage = `Sukses menyimpan ${data.inserted} baris ke dalam sk_bed.`;
          // Reset form on success
          setTimeout(() => {
            this.step = 2;               // Kembali ke Step 2 (bukan 1) agar data tidak hilang
            this.importMessage = '';     // Hilangkan pesan
            this.importSuccess = false;
            this.checkActiveSK();
          }, 3000);
        } else {
          this.importSuccess = false;
          this.importMessage = `Gagal: ${data.error}`;
        }
      } catch (err) {
        this.importSuccess = false;
        this.importMessage = `Error Network: ${err.message}`;
      } finally {
        this.importing = false;
      }
    }
  }));
});
