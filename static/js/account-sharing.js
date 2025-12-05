// Account Sharing Alpine.js Component
function accountSharing() {
    return {
        members: [],
        invitations: [],
        inviting: false,
        inviteFeedback: '',
        invitationToken: '',
        invitationLink: '',
        copied: false,
        currentUserID: 0,
        currentUserRole: '',

        init() {
            this.loadMembers();
            this.loadInvitations();
        },

        async loadMembers() {
            try {
                const response = await fetch('/api/account/members', {
                    headers: { 'X-CSRF-Token': document.querySelector('meta[name=csrf-token]').content }
                });

                if (!response.ok) throw new Error('Failed to load members');

                this.members = await response.json();

                // Find current user's role
                const currentMember = this.members.find(m => m.UserID === this.currentUserID);
                if (currentMember) {
                    this.currentUserRole = currentMember.Role;
                }

                this.renderMembers();
            } catch (error) {
                document.getElementById('members-list').innerHTML =
                    '<p style="color: var(--danger-primary);">Error loading members</p>';
            }
        },

        renderMembers() {
            const html = this.members.map(member => `
            <div style="display: flex; justify-content: space-between; align-items: center; padding: var(--space-3); background: var(--color-surface); border: 1px solid var(--color-border); border-radius: var(--radius-md); margin-bottom: var(--space-2);">
                <div>
                    <strong>${member.Username || 'Unknown User'}</strong>
                    ${member.Role === 'owner' ? '<span class="badge" style="margin-left: 0.5rem; background: var(--brand-primary-bg); color: var(--brand-primary); padding: 2px 8px; font-size: 0.75rem; border-radius: 999px;">Owner</span>' : ''}
                    ${member.UserID === this.currentUserID ? '<span class="badge" style="margin-left: 0.5rem; background: var(--color-bg-tertiary); color: var(--color-text-secondary); padding: 2px 8px; font-size: 0.75rem; border-radius: 999px;">You</span>' : ''}
                </div>
                ${member.UserID !== this.currentUserID && this.currentUserRole === 'owner' ? `
                    <button type="button"
                            class="btn-sm outline secondary"
                            style="margin: 0; color: var(--danger-primary); border-color: var(--danger-border);"
                            onclick="removeMember(${member.UserID}, '${member.Username}')">
                        Remove
                    </button>
                ` : ''}
            </div>
        `).join('');

            document.getElementById('members-list').innerHTML =
                html || '<p style="color: var(--color-text-muted);">No members found</p>';
        },

        async loadInvitations() {
            try {
                const response = await fetch('/api/invitations', {
                    headers: { 'X-CSRF-Token': document.querySelector('meta[name=csrf-token]').content }
                });

                if (!response.ok) {
                    // Don't show error for empty list
                    this.invitations = [];
                    this.renderInvitations();
                    return;
                }

                const data = await response.json();
                this.invitations = data || [];
                this.renderInvitations();
            } catch (error) {
                // Show empty list instead of error
                this.invitations = [];
                this.renderInvitations();
            }
        },

        renderInvitations() {
            if (!this.invitations || this.invitations.length === 0) {
                document.getElementById('invitations-list').innerHTML =
                    '<p style="color: var(--color-text-muted);">No invitations</p>';
                return;
            }

            const html = this.invitations.map(inv => {
                const accepted = inv.accepted_at ? `<span class="text-success" style="margin-left: 0.5rem;">(Accepted ${new Date(inv.accepted_at).toLocaleDateString()})</span>` : '';

                // Get stored token from localStorage for pending invitations
                const storedToken = !inv.accepted_at ? localStorage.getItem(`invite_token_${inv.id}`) : null;
                const inviteLink = storedToken ? `${window.location.origin}/register?invite=${storedToken}` : null;

                return `
            <div style="padding: var(--space-3); background: var(--color-surface); border: 1px solid var(--color-border); border-radius: var(--radius-md); margin-bottom: var(--space-2);">
                <div style="display: flex; justify-content: space-between; align-items: center;">
                    <div>
                        <strong>Invitation ${accepted}</strong>
                        <br><small style="color: var(--color-text-muted);">
                            Created ${new Date(inv.created_at).toLocaleDateString()} -
                            Expires ${new Date(inv.expires_at).toLocaleDateString()}
                        </small>
                    </div>
                    ${!inv.accepted_at ? `
                    <button type="button"
                            class="btn-sm outline secondary"
                            style="margin: 0; color: var(--danger-primary); border-color: var(--danger-border);"
                            onclick="confirmRevokeInvitation(${inv.id})">
                        Revoke
                    </button>
                    ` : ''}
                </div>
                ${inviteLink ? `
                <div style="margin-top: var(--space-3); padding-top: var(--space-3); border-top: 1px solid var(--color-border);">
                    <small style="color: var(--color-text-muted); display: block; margin-bottom: var(--space-2);">Invitation Link:</small>
                    <div style="display: flex; gap: var(--space-2); align-items: center;">
                        <input type="text"
                               value="${inviteLink}"
                               readonly
                               onclick="this.select()"
                               style="flex: 1; font-family: monospace; font-size: 0.85rem; margin: 0;">
                        <button type="button"
                                class="btn-sm outline"
                                onclick="copyToClipboard('${inviteLink}', this)"
                                style="margin: 0; white-space: nowrap;">
                            Copy Link
                        </button>
                    </div>
                </div>
                ` : ''}
            </div>
            `;
            }).join('');

            document.getElementById('invitations-list').innerHTML = html;
        },

        async sendInvitation() {
            this.inviting = true;
            this.inviteFeedback = '';
            this.invitationToken = '';
            this.invitationLink = '';

            try {
                const response = await fetch('/api/invitations', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'X-CSRF-Token': document.querySelector('meta[name=csrf-token]').content
                    },
                    body: JSON.stringify({
                        email: 'invitation@localhost',
                        role: 'member'
                    })
                });

                if (!response.ok) {
                    const error = await response.text();
                    throw new Error(error || 'Failed to create invitation');
                }

                const data = await response.json();
                this.invitationToken = data.token;
                this.invitationLink = `${window.location.origin}/register?invite=${data.token}`;

                // Store token in localStorage so we can display it later
                localStorage.setItem(`invite_token_${data.id}`, data.token);

                this.inviteFeedback = '<div class="alert-success">Invitation link created successfully!</div>';

                // Reload invitations list
                await this.loadInvitations();

                // Clear feedback after 5 seconds
                setTimeout(() => {
                    this.inviteFeedback = '';
                }, 5000);

            } catch (error) {
                this.inviteFeedback = `<div class="alert-danger">Error: ${error.message}</div>`;
            } finally {
                this.inviting = false;
            }
        },

        copyInvitationLink() {
            navigator.clipboard.writeText(this.invitationLink);
            this.copied = true;
            setTimeout(() => {
                this.copied = false;
            }, 2000);
        }
    }
}

// Helper functions that need to be global
async function removeMember(userId, username) {
    const modal = document.getElementById('remove-member-modal');
    document.getElementById('remove-member-name').textContent = username;
    modal.showModal();

    // Store for confirmation
    window.pendingRemoveUserId = userId;
}

async function confirmRemoveMember() {
    const modal = document.getElementById('remove-member-modal');
    const userId = window.pendingRemoveUserId;

    try {
        const response = await fetch(`/api/account/members/${userId}`, {
            method: 'DELETE',
            headers: {
                'X-CSRF-Token': document.querySelector('meta[name=csrf-token]').content
            }
        });

        if (!response.ok) throw new Error('Failed to remove member');

        modal.close();
        window.location.reload();
    } catch (error) {
        modal.close();
        alert('Error removing member: ' + error.message);
    }
}

function confirmRevokeInvitation(invitationId) {
    const modal = document.getElementById('revoke-invitation-modal');
    modal.showModal();

    // Store for confirmation
    window.pendingRevokeInvitationId = invitationId;
}

async function revokeInvitation() {
    const modal = document.getElementById('revoke-invitation-modal');
    const invitationId = window.pendingRevokeInvitationId;

    try {
        const response = await fetch(`/api/invitations/${invitationId}`, {
            method: 'DELETE',
            headers: {
                'X-CSRF-Token': document.querySelector('meta[name=csrf-token]').content
            }
        });

        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(errorText || 'Failed to revoke invitation');
        }

        // Remove token from localStorage
        localStorage.removeItem(`invite_token_${invitationId}`);

        modal.close();
        window.location.reload();
    } catch (error) {
        modal.close();
        alert('Error revoking invitation: ' + error.message);
    }
}

function copyToClipboard(text, button) {
    navigator.clipboard.writeText(text).then(() => {
        const originalText = button.textContent;
        button.textContent = 'Copied!';
        setTimeout(() => {
            button.textContent = originalText;
        }, 2000);
    });
}
