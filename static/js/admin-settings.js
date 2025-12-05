// Admin Settings Alpine.js Component
function adminSettings() {
    return {
        smtp: { host: '', port: 587, username: '', password: '', from_name: 'P-TRACK', from_email: '', enabled: false },
        site: { site_url: '', site_title: 'P-TRACK', site_description: '' },
        stats: {},
        users: [],
        accounts: [],
        myAccountId: 0,
        backups: [],
        autoBackup: { enabled: false, frequency: 'daily', keep_count: 7, last_run: '' },
        backupFeedback: '',
        creatingBackup: false,
        feedback: '',
        siteFeedback: '',
        usersFeedback: '',
        accountsFeedback: '',
        saving: false,
        savingSite: false,
        testing: false,
        pendingAction: null,
        testEmail: '',

        showConfirmModal(title, message, confirmText, onConfirm) {
            document.getElementById('admin-modal-title').textContent = title;
            document.getElementById('admin-modal-message').textContent = message;
            document.getElementById('admin-modal-confirm-btn').textContent = confirmText || 'Confirm';
            this.pendingAction = onConfirm;
            document.getElementById('admin-confirm-modal').showModal();
        },

        closeConfirmModal() {
            this.pendingAction = null;
            document.getElementById('admin-confirm-modal').close();
        },

        async executeConfirmAction() {
            if (this.pendingAction) {
                await this.pendingAction();
            }
            this.closeConfirmModal();
        },

        async loadSettings() {
            try {
                const csrf = document.querySelector('meta[name=csrf-token]').content;
                const r = await fetch('/api/admin/settings', { headers: { 'X-CSRF-Token': csrf } });
                if (r.ok) {
                    const d = await r.json();
                    this.smtp = { ...this.smtp, ...d.smtp };
                    if (d.site) this.site = { ...this.site, ...d.site };
                    this.stats = d.site_stats || {};
                    this.myAccountId = d.account_id || 0;
                }
                await this.loadUsers();
                await this.loadAccounts();
                await this.loadBackups();
                await this.loadAutoBackupSettings();
            } catch (e) {
                console.error('Failed to load admin settings:', e);
            }
        },

        async loadUsers() {
            try {
                const r = await fetch('/api/admin/users', { headers: { 'X-CSRF-Token': document.querySelector('meta[name=csrf-token]').content } });
                if (r.ok) this.users = await r.json();
            } catch (e) {
                console.error('Failed to load users:', e);
            }
        },

        async loadAccounts() {
            try {
                const r = await fetch('/api/admin/accounts', { headers: { 'X-CSRF-Token': document.querySelector('meta[name=csrf-token]').content } });
                if (r.ok) this.accounts = await r.json();
            } catch (e) {
                console.error('Failed to load accounts:', e);
            }
        },

        async saveSmtpSettings() {
            this.saving = true;
            this.feedback = '';
            try {
                const r = await fetch('/api/admin/smtp', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': document.querySelector('meta[name=csrf-token]').content },
                    body: JSON.stringify(this.smtp)
                });
                this.feedback = r.ok ? '<div class="alert-success">SMTP settings saved!</div>' : '<div class="alert-danger">' + await r.text() + '</div>';
                if (r.ok) this.smtp.password = '';
            } catch (e) {
                this.feedback = '<div class="alert-danger">Error: ' + e.message + '</div>';
            } finally {
                this.saving = false;
                setTimeout(() => this.feedback = '', 5000);
            }
        },

        async saveSiteSettings() {
            this.savingSite = true;
            this.siteFeedback = '';
            try {
                const r = await fetch('/api/admin/site', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': document.querySelector('meta[name=csrf-token]').content },
                    body: JSON.stringify(this.site)
                });
                this.siteFeedback = r.ok ? '<div class="alert-success">Site settings saved!</div>' : '<div class="alert-danger">' + await r.text() + '</div>';
            } catch (e) {
                this.siteFeedback = '<div class="alert-danger">Error: ' + e.message + '</div>';
            } finally {
                this.savingSite = false;
                setTimeout(() => this.siteFeedback = '', 5000);
            }
        },

        testSmtp() {
            this.testEmail = '';
            document.getElementById('smtp-test-modal').showModal();
        },

        async sendTestEmail() {
            const email = this.testEmail.trim();
            if (!email) return;

            document.getElementById('smtp-test-modal').close();
            this.testing = true;
            this.feedback = '';
            try {
                const r = await fetch('/api/admin/smtp/test', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': document.querySelector('meta[name=csrf-token]').content },
                    body: JSON.stringify({ email })
                });
                const d = await r.json();
                this.feedback = r.ok ? '<div class="alert-success">' + d.message + '</div>' : '<div class="alert-danger">' + d.message + '</div>';
            } catch (e) {
                this.feedback = '<div class="alert-danger">Error: ' + e.message + '</div>';
            } finally {
                this.testing = false;
                setTimeout(() => this.feedback = '', 5000);
            }
        },

        toggleUserStatus(user) {
            const action = user.is_active ? 'deactivate' : 'activate';
            const actionTitle = user.is_active ? 'Deactivate' : 'Activate';
            this.showConfirmModal(
                actionTitle + ' User',
                'Are you sure you want to ' + action + ' ' + user.username + '?',
                actionTitle + ' User',
                async () => {
                    try {
                        const r = await fetch('/api/admin/users/status', {
                            method: 'PUT',
                            headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': document.querySelector('meta[name=csrf-token]').content },
                            body: JSON.stringify({ user_id: user.id, active: !user.is_active })
                        });
                        if (r.ok) {
                            user.is_active = !user.is_active;
                            this.usersFeedback = '<div class="alert-success">User status updated!</div>';
                        } else {
                            this.usersFeedback = '<div class="alert-danger">' + await r.text() + '</div>';
                        }
                    } catch (e) {
                        this.usersFeedback = '<div class="alert-danger">Error: ' + e.message + '</div>';
                    }
                    setTimeout(() => this.usersFeedback = '', 5000);
                }
            );
        },

        deleteUser(user) {
            this.showConfirmModal(
                'Delete User',
                'Permanently delete user ' + user.username + '? This cannot be undone.',
                'Delete User',
                async () => {
                    try {
                        const r = await fetch('/api/admin/users', {
                            method: 'DELETE',
                            headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': document.querySelector('meta[name=csrf-token]').content },
                            body: JSON.stringify({ user_id: user.id })
                        });
                        if (r.ok) {
                            await this.loadUsers();
                            await this.loadAccounts();
                            this.usersFeedback = '<div class="alert-success">User deleted!</div>';
                        } else {
                            this.usersFeedback = '<div class="alert-danger">' + await r.text() + '</div>';
                        }
                    } catch (e) {
                        this.usersFeedback = '<div class="alert-danger">Error: ' + e.message + '</div>';
                    }
                    setTimeout(() => this.usersFeedback = '', 5000);
                }
            );
        },

        deleteAccount(account) {
            this.showConfirmModal(
                'Delete Account',
                'Delete account ' + account.name + '? This will permanently delete ALL data for all users in this account!',
                'Delete Account',
                async () => {
                    try {
                        const r = await fetch('/api/admin/accounts', {
                            method: 'DELETE',
                            headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': document.querySelector('meta[name=csrf-token]').content },
                            body: JSON.stringify({ account_id: account.id })
                        });
                        if (r.ok) {
                            await this.loadAccounts();
                            await this.loadUsers();
                            this.accountsFeedback = '<div class="alert-success">Account deleted!</div>';
                        } else {
                            this.accountsFeedback = '<div class="alert-danger">' + await r.text() + '</div>';
                        }
                    } catch (e) {
                        this.accountsFeedback = '<div class="alert-danger">Error: ' + e.message + '</div>';
                    }
                    setTimeout(() => this.accountsFeedback = '', 5000);
                }
            );
        },

        async loadBackups() {
            try {
                const r = await fetch('/api/admin/backups', { headers: { 'X-CSRF-Token': document.querySelector('meta[name=csrf-token]').content } });
                if (r.ok) this.backups = await r.json();
            } catch (e) {
                console.error('Failed to load backups:', e);
            }
        },

        async loadAutoBackupSettings() {
            try {
                const r = await fetch('/api/admin/backups/auto', { headers: { 'X-CSRF-Token': document.querySelector('meta[name=csrf-token]').content } });
                if (r.ok) this.autoBackup = await r.json();
            } catch (e) {
                console.error('Failed to load auto-backup settings:', e);
            }
        },

        async saveAutoBackupSettings() {
            try {
                const r = await fetch('/api/admin/backups/auto', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': document.querySelector('meta[name=csrf-token]').content },
                    body: JSON.stringify(this.autoBackup)
                });
                if (r.ok) {
                    const d = await r.json();
                    this.autoBackup = d.settings;
                    this.backupFeedback = '<div class="alert-success">Auto-backup settings saved!</div>';
                } else {
                    this.backupFeedback = '<div class="alert-danger">' + await r.text() + '</div>';
                }
            } catch (e) {
                this.backupFeedback = '<div class="alert-danger">Error: ' + e.message + '</div>';
            }
            setTimeout(() => this.backupFeedback = '', 3000);
        },

        async createBackup() {
            this.creatingBackup = true;
            this.backupFeedback = '';
            try {
                const r = await fetch('/api/admin/backups', { method: 'POST', headers: { 'X-CSRF-Token': document.querySelector('meta[name=csrf-token]').content } });
                if (r.ok) {
                    const d = await r.json();
                    this.backupFeedback = '<div class="alert-success">' + d.message + '</div>';
                    await this.loadBackups();
                } else {
                    this.backupFeedback = '<div class="alert-danger">' + await r.text() + '</div>';
                }
            } catch (e) {
                this.backupFeedback = '<div class="alert-danger">Error: ' + e.message + '</div>';
            } finally {
                this.creatingBackup = false;
                setTimeout(() => this.backupFeedback = '', 5000);
            }
        },

        deleteBackup(backup) {
            this.showConfirmModal(
                'Delete Backup',
                'Delete backup ' + backup.filename + '?',
                'Delete Backup',
                async () => {
                    try {
                        const r = await fetch('/api/admin/backups', {
                            method: 'DELETE',
                            headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': document.querySelector('meta[name=csrf-token]').content },
                            body: JSON.stringify({ filename: backup.filename })
                        });
                        if (r.ok) {
                            await this.loadBackups();
                            this.backupFeedback = '<div class="alert-success">Backup deleted!</div>';
                        } else {
                            this.backupFeedback = '<div class="alert-danger">' + await r.text() + '</div>';
                        }
                    } catch (e) {
                        this.backupFeedback = '<div class="alert-danger">Error: ' + e.message + '</div>';
                    }
                    setTimeout(() => this.backupFeedback = '', 5000);
                }
            );
        },

        async uploadBackup(event) {
            const file = event.target.files[0];
            if (!file) return;
            const fd = new FormData();
            fd.append('backup', file);
            this.backupFeedback = '<div class="alert-info">Uploading...</div>';
            try {
                const r = await fetch('/api/admin/backups/upload', {
                    method: 'POST',
                    headers: { 'X-CSRF-Token': document.querySelector('meta[name=csrf-token]').content },
                    body: fd
                });
                if (r.ok) {
                    const d = await r.json();
                    this.backupFeedback = '<div class="alert-success">' + d.message + '</div>';
                    await this.loadBackups();
                } else {
                    this.backupFeedback = '<div class="alert-danger">' + await r.text() + '</div>';
                }
            } catch (e) {
                this.backupFeedback = '<div class="alert-danger">Error: ' + e.message + '</div>';
            }
            event.target.value = '';
            setTimeout(() => this.backupFeedback = '', 5000);
        },

        restoreBackup(backup) {
            this.showConfirmModal(
                'Restore Backup',
                'Restore from backup ' + backup.filename + '? The server will restart.',
                'Restore Backup',
                async () => {
                    this.backupFeedback = '<div class="alert-info">Restoring backup...</div>';
                    try {
                        const r = await fetch('/api/admin/backups/restore', {
                            method: 'POST',
                            headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': document.querySelector('meta[name=csrf-token]').content },
                            body: JSON.stringify({ filename: backup.filename, confirm: true })
                        });
                        if (r.ok) {
                            this.backupFeedback = '<div class="alert-success">Restore initiated. Refreshing...</div>';
                            setTimeout(() => location.reload(), 5000);
                        } else {
                            this.backupFeedback = '<div class="alert-danger">' + await r.text() + '</div>';
                        }
                    } catch (e) {
                        this.backupFeedback = '<div class="alert-info">Server restarting. Please refresh...</div>';
                        setTimeout(() => location.reload(), 5000);
                    }
                }
            );
        }
    }
}
