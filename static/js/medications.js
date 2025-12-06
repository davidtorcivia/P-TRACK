/**
 * Medications Management JavaScript
 * Handles medication CRUD operations, logging, and extensive modal management
 * Replaces inline event handlers for CSP compliance
 */

document.addEventListener('DOMContentLoaded', function () {
    // Get CSRF token from meta tag
    function getCSRFToken() {
        const metaTag = document.querySelector('meta[name="csrf-token"]');
        return metaTag ? metaTag.getAttribute('content') : '';
    }

    // --- Modal Helpers ---
    function openModal(modalId) {
        const modal = document.getElementById(modalId);
        if (modal) modal.showModal();
    }

    function closeModal(modalId) {
        const modal = document.getElementById(modalId);
        if (modal) modal.close();
    }

    // Global click handler for modal close buttons
    document.querySelectorAll('dialog button[aria-label="Close"], dialog button.secondary').forEach(btn => {
        // Find the closest dialog parent
        const dialog = btn.closest('dialog');
        // Only attach if it's explicitly a close action (prev or cancel)
        if (dialog && (btn.getAttribute('rel') === 'prev' || btn.textContent === 'Cancel')) {
            btn.addEventListener('click', function () {
                dialog.close();
            });
        }
    });

    // --- Add Medication ---
    const addMedBtn = document.getElementById('btn-add-medication');
    if (addMedBtn) {
        addMedBtn.addEventListener('click', () => openModal('new-medication'));
    }

    const newMedForm = document.querySelector('#new-medication form');
    if (newMedForm) {
        newMedForm.addEventListener('submit', function (e) {
            e.preventDefault();
            handleMedicationForm(this, '/api/medications', 'POST');
        });
    }

    // --- Edit Medication ---
    // Open edit modal
    document.querySelectorAll('[data-action="edit-medication"]').forEach(btn => {
        btn.addEventListener('click', function () {
            const medId = this.getAttribute('data-id');
            openModal('edit-medication-' + medId);
        });
    });

    // Handle edit forms
    document.querySelectorAll('[data-form="edit-medication"]').forEach(form => {
        form.addEventListener('submit', function (e) {
            e.preventDefault();
            const medId = this.getAttribute('data-id');
            handleMedicationForm(this, '/api/medications/' + medId, 'PUT');
        });
    });

    // Generic form handler for Add/Edit
    function handleMedicationForm(form, url, method) {
        const btn = form.querySelector('button[type=submit]');
        btn.disabled = true;
        btn.setAttribute('aria-busy', 'true');

        const formData = new FormData(form);
        const data = {
            name: formData.get('name'),
            dosage: formData.get('dosage'),
            frequency_type: formData.get('frequency_type'),
            frequency_value: formData.get('frequency_value'),
            reminder_time: formData.get('reminder_time'),
            notes: formData.get('notes') || null
        };

        // For edit, we might need to explicitly set is_active if not present, 
        // but the Go backend might handle partial updates or we send what's needed.
        // The original inline JS sent `is_active: true` for edits, let's keep that consistency if needed,
        // though usually edit doesn't change active status unless explicitly requested.
        // Looking at original code: `is_active: true` was sent.
        if (method === 'PUT') {
            data.is_active = true;
        }

        // Build frequency string
        if (data.frequency_type === 'daily') {
            data.frequency = 'Every day';
        } else if (data.frequency_type === 'hours') {
            data.frequency = 'Every ' + data.frequency_value + ' hours';
        } else if (data.frequency_type === 'days') {
            data.frequency = 'Every ' + data.frequency_value + ' days';
        } else {
            data.frequency = formData.get('frequency_custom');
        }

        fetch(url, {
            method: method,
            headers: {
                'Content-Type': 'application/json',
                'X-CSRF-Token': getCSRFToken()
            },
            body: JSON.stringify(data)
        })
            .then(response => {
                if (response.ok) {
                    window.location.reload();
                } else {
                    return response.text().then(text => {
                        console.error('Error:', text);
                        alert('Error: ' + text);
                        btn.disabled = false;
                        btn.removeAttribute('aria-busy');
                    });
                }
            })
            .catch(error => {
                console.error('Error:', error.message);
                alert('Error: ' + error.message);
                btn.disabled = false;
                btn.removeAttribute('aria-busy');
            });
    }

    // --- Log Medication (Take/Miss) ---
    document.querySelectorAll('[data-action="log-medication"]').forEach(btn => {
        btn.addEventListener('click', function () {
            const medId = this.getAttribute('data-id');
            const taken = this.getAttribute('data-taken') === 'true'; // If button says "Mark Taken", we send true? 
            // Wait, logic in HTML was: `if .TakenToday` then button says "Mark Missed" and sets taken=false.
            // So data-taken should be the *value to send*.

            fetch('/api/medications/' + medId + '/log', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-CSRF-Token': getCSRFToken()
                },
                body: JSON.stringify({ taken: taken })
            })
                .then(response => {
                    if (response.ok) window.location.reload();
                    else console.error('Error logging medication');
                })
                .catch(err => console.error('Error:', err));
        });
    });

    // --- Deactivation & Deletion State ---
    let currentMedId = null;
    let currentMedName = null;

    // --- Deactivate ---
    // Trigger confirmation
    document.querySelectorAll('[data-action="confirm-deactivate"]').forEach(btn => {
        btn.addEventListener('click', function () {
            currentMedId = this.getAttribute('data-id');
            currentMedName = this.getAttribute('data-name');

            // Close edit modal if open (it might be open behind)
            // The button logic in HTML was inside the edit modal.
            // So we just open the confirm modal.

            const nameEl = document.getElementById('deactivate-med-name');
            if (nameEl) nameEl.textContent = currentMedName;

            openModal('deactivate-medication-confirm');
        });
    });

    // Execute deactivation
    const confirmDeactivateBtn = document.getElementById('btn-confirm-deactivate');
    if (confirmDeactivateBtn) {
        confirmDeactivateBtn.addEventListener('click', function () {
            if (!currentMedId) return;

            // Close edit modal if it's open
            const editModal = document.getElementById('edit-medication-' + currentMedId);
            if (editModal && editModal.open) editModal.close();

            fetch('/api/medications/' + currentMedId, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                    'X-CSRF-Token': getCSRFToken()
                },
                body: JSON.stringify({ is_active: false })
            })
                .then(response => {
                    if (response.ok) window.location.reload();
                    else console.error('Error deactivating medication');
                });

            closeModal('deactivate-medication-confirm');
        });
    }

    // --- Reactivate ---
    document.querySelectorAll('[data-action="reactivate-medication"]').forEach(btn => {
        btn.addEventListener('click', function () {
            const medId = this.getAttribute('data-id');
            const name = this.getAttribute('data-name');
            const dosage = this.getAttribute('data-dosage');
            const frequency = this.getAttribute('data-frequency');

            fetch('/api/medications/' + medId, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                    'X-CSRF-Token': getCSRFToken()
                },
                body: JSON.stringify({
                    is_active: true,
                    name: name,
                    dosage: dosage,
                    frequency: frequency
                })
            })
                .then(response => {
                    if (response.ok) window.location.reload();
                    else console.error('Error reactivating medication');
                });
        });
    });

    // --- Delete ---
    // Trigger confirmation
    document.querySelectorAll('[data-action="confirm-delete"]').forEach(btn => {
        btn.addEventListener('click', function () {
            currentMedId = this.getAttribute('data-id');
            currentMedName = this.getAttribute('data-name');

            const nameEl = document.getElementById('delete-med-name');
            if (nameEl) nameEl.textContent = currentMedName;

            openModal('delete-medication-confirm');
        });
    });

    // Execute deletion
    const confirmDeleteBtn = document.getElementById('btn-confirm-delete');
    if (confirmDeleteBtn) {
        confirmDeleteBtn.addEventListener('click', function () {
            if (!currentMedId) return;

            // Close edit modal if open
            const editModal = document.getElementById('edit-medication-' + currentMedId);
            if (editModal && editModal.open) editModal.close();

            fetch('/api/medications/' + currentMedId, {
                method: 'DELETE',
                headers: { 'X-CSRF-Token': getCSRFToken() }
            })
                .then(response => {
                    if (response.ok) window.location.reload();
                    else console.error('Error deleting medication');
                });

            closeModal('delete-medication-confirm');
        });
    }
});
