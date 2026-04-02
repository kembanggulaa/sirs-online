function bedsManagement() {
  return {
    classRooms: [],
    selectedClassRoom: '',
    
    loadingData: false,
    showTable: false,
    mode: 'new', // 'new' | 'edit'
    
    kamarsData: [], // Array of { kamar: 'VIP', defaults: {...}, rows: [...] }
    
    saving: false,
    saveMsg: '',
    saveSuccess: false,
    
    generateUUID() {
        return Date.now().toString(36) + Math.random().toString(36).substring(2);
    },
    
    init() {
      this.fetchClassRooms();
    },
    
    async fetchClassRooms() {
      try {
        const res = await fetch('/api/beds/rooms');
        const data = await res.json();
        this.classRooms = data || [];
      } catch (err) {
        console.error("Gagal load classRooms", err);
      }
    },
    
    async loadBedsData() {
      if (!this.selectedClassRoom) return;
      
      this.loadingData = true;
      this.saveMsg = '';
      this.showTable = false;
      
      try {
        const url = `/api/beds/by-room?class_room_id=${encodeURIComponent(this.selectedClassRoom)}`;
        const res = await fetch(url);
        const data = await res.json();
        
        this.mode = data.mode || 'new';
        this.kamarsData = data.kamars || [];
        
        // Inject UI ID for animation/scrolling & provide empty row if needed
        for (let kg of this.kamarsData) {
            kg._ui_id = this.generateUUID();
            if (!kg.rows || kg.rows.length === 0) {
                kg.rows = [{
                    bed_id: '', room_id: '', id_kelas: '', nm_kelas: '', 
                    id_perawatan: '', nm_perawatan: '', 
                    id_tt_siranap: kg.defaults ? kg.defaults.id_tt_siranap : '', 
                    id_siranap: '', deskripsi_siranap: '', 
                    covid: kg.defaults ? kg.defaults.covid : '0',
                    _ui_id: this.generateUUID()
                }];
            } else {
                for (let r of kg.rows) {
                    r._ui_id = r._ui_id || this.generateUUID();
                }
            }
        }
        
        if (this.kamarsData.length === 0) {
          this.addKamarGroup();
        }
        
        this.showTable = true;
      } catch (err) {
        console.error("Gagal memuat data beds", err);
        alert("Terjadi kesalahan saat mengambil data Beds.");
      } finally {
        this.loadingData = false;
      }
    },
    
    addKamarGroup() {
       const kgId = this.generateUUID();
       this.kamarsData.push({
           _ui_id: kgId,
           kamar: '',
           defaults: { id_tt_siranap: '', covid: '0' },
           rows: [{
               _ui_id: this.generateUUID(),
               bed_id: '', room_id: '', id_kelas: '', nm_kelas: '', 
               id_perawatan: '', nm_perawatan: '', id_tt_siranap: '', 
               id_siranap: '', deskripsi_siranap: '', covid: '0'
           }]
       });
       
       this.$nextTick(() => {
         const el = document.getElementById('kg-' + kgId);
         if (el) {
           el.scrollIntoView({ behavior: 'smooth', block: 'center' });
           el.classList.add('ring-2', 'ring-brand-500', 'ring-offset-2');
           setTimeout(() => el.classList.remove('ring-2', 'ring-brand-500', 'ring-offset-2'), 1500);
         }
       });
    },
    
    removeKamarGroup(kgIdx) {
       if (confirm('Yakin ingin menghapus seluruh grup kamar ini bersama baris-baris table di dalamnya?')) {
           this.kamarsData.splice(kgIdx, 1);
       }
    },
    
    addRowToKamar(kgIdx) {
      const kg = this.kamarsData[kgIdx];
      const newId = this.generateUUID();
      if (kg.rows.length > 0) {
        const lastRow = kg.rows[kg.rows.length - 1];
        kg.rows.push({
          ...lastRow,
          bed_id: '',
          _ui_id: newId
        });
      } else {
        kg.rows.push({
          bed_id: '',
          room_id: '',
          id_kelas: '',
          nm_kelas: '',
          id_perawatan: '',
          nm_perawatan: '',
          id_tt_siranap: kg.defaults && kg.defaults.id_tt_siranap ? kg.defaults.id_tt_siranap : '',
          id_siranap: '',
          deskripsi_siranap: '',
          covid: kg.defaults && kg.defaults.covid ? kg.defaults.covid : '0',
          _ui_id: newId
        });
      }
      
      this.$nextTick(() => {
        const el = document.getElementById('row-' + newId);
        if (el) {
          el.scrollIntoView({ behavior: 'smooth', block: 'center' });
          el.classList.add('bg-brand-100');
          setTimeout(() => el.classList.remove('bg-brand-100'), 1500);
        }
      });
    },
    
    removeRowFromKamar(kgIdx, idx) {
      if (confirm('Yakin ingin menghapus baris data ini dari layar?\n\n(Catatan: Data di database hanya akan terhapus setelah Anda menekan tombol "Simpan Data Beds")')) {
        this.kamarsData[kgIdx].rows.splice(idx, 1);
      }
    },
    
    async saveBeds() {
      // Validate and flatten
      const allRows = [];
      for (const kg of this.kamarsData) {
        for (let i = 0; i < kg.rows.length; i++) {
          let r = kg.rows[i];
          if (!r.bed_id || r.bed_id === '') {
            this.saveSuccess = false;
            this.saveMsg = `❌ bed_id harus diisi di kamar '${kg.kamar || "Tak bernama"}' baris ${i+1}.`;
            return;
          }
          if (!r.id_kelas || !r.nm_kelas) {
            this.saveSuccess = false;
            this.saveMsg = `❌ id_kelas dan nm_kelas harus diisi di kamar '${kg.kamar || "Tak bernama"}' baris ${i+1}.`;
            return;
          }
          
          allRows.push({
            ...r,
            bed_id: parseInt(r.bed_id, 10),
            kamar: kg.kamar
          });
        }
      }
      
      const payload = {
        class_room_id: this.selectedClassRoom,
        rows: allRows
      };
      
      this.saving = true;
      this.saveMsg = '';
      
      try {
        const res = await fetch('/api/beds/upsert', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        });
        
        const data = await res.json();
        
        if (!res.ok) {
          throw new Error(data.error || 'Terjadi kesalahan pada server');
        }
        
        window.isFormDirty = false;
        this.saveSuccess = true;
        let pMsg = `✅ Berhasil disimpan: ${data.inserted || 0} ditambahkan, ${data.updated || 0} di-update.`;
        if (data.deleted && data.deleted > 0) {
          pMsg += ` (${data.deleted} dihapus dari server).`;
        }
        this.saveMsg = pMsg;
        this.mode = 'edit'; // Since it's saved, future saves on this view are edits
        
      } catch (err) {
        console.error("Gagal menyimpan beds", err);
        this.saveSuccess = false;
        this.saveMsg = '❌ Gagal: ' + err.message;
      } finally {
        this.saving = false;
      }
    }
  };
}
