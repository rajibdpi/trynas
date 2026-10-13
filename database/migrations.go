package database

func (a *Module) MigrateV5() error {
	_, err := a.DB.Exec(`CREATE TABLE IF NOT EXISTS registration_requests(id INTEGER PRIMARY KEY, username TEXT UNIQUE NOT NULL, password_hash TEXT NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
 CREATE TABLE IF NOT EXISTS user_profiles(user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,display_name TEXT NOT NULL DEFAULT '',email TEXT NOT NULL DEFAULT '');
 CREATE TABLE IF NOT EXISTS user_homes(user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,path TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS user_limits(user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,quota_bytes INTEGER NOT NULL DEFAULT 0 CHECK(quota_bytes>=0));
 CREATE TABLE IF NOT EXISTS permissions(user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,path TEXT NOT NULL,access TEXT NOT NULL CHECK(access IN ('none','read','write')),PRIMARY KEY(user_id,path));
 CREATE TABLE IF NOT EXISTS file_owners(path TEXT PRIMARY KEY,user_id INTEGER NOT NULL REFERENCES users(id),bytes INTEGER NOT NULL CHECK(bytes>=0));
 CREATE TABLE IF NOT EXISTS branding_assets(key TEXT PRIMARY KEY,mime TEXT NOT NULL,data BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS audit_log_ips(log_id INTEGER PRIMARY KEY REFERENCES audit_logs(id) ON DELETE CASCADE,ip TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS app_settings(key TEXT PRIMARY KEY,value TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS remote_trash(id INTEGER PRIMARY KEY,user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,storage TEXT NOT NULL,original_path TEXT NOT NULL,trash_path TEXT NOT NULL,deleted_at DATETIME DEFAULT CURRENT_TIMESTAMP);
 CREATE TABLE IF NOT EXISTS share_storage(share_id INTEGER PRIMARY KEY REFERENCES shares(id) ON DELETE CASCADE,storage TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS share_items(share_id INTEGER NOT NULL REFERENCES shares(id) ON DELETE CASCADE,path TEXT NOT NULL,PRIMARY KEY(share_id,path));
 CREATE TABLE IF NOT EXISTS share_passwords(share_id INTEGER PRIMARY KEY REFERENCES shares(id) ON DELETE CASCADE,password_hash TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS share_sessions(token TEXT PRIMARY KEY,share_id INTEGER NOT NULL REFERENCES shares(id) ON DELETE CASCADE,expires_at TEXT NOT NULL);
 CREATE INDEX IF NOT EXISTS idx_file_owners_user_bytes ON file_owners(user_id,bytes);
 CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
 CREATE INDEX IF NOT EXISTS idx_share_sessions_share ON share_sessions(share_id);`)
	if err != nil {
		return err
	}
	return a.NormalizeShareExpiries()
}
