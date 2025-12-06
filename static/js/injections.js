/**
 * Injection Logging JavaScript
 * Handles injection CRUD operations with proper event listeners for CSP compliance
 */

document.addEventListener('DOMContentLoaded', function () {
    // Get CSRF token from meta tag
    function getCSRFToken() {
        const metaTag = document.querySelector('meta[name="csrf-token"]');
        return metaTag ? metaTag.getAttribute('content') : '';
    }

    // Helper to format date for input[type="date"]
    function formatDate(date) {
        const d = new Date(date);
        let month = '' + (d.getMonth() + 1);
        let day = '' + d.getDate();
        const year = d.getFullYear();

        if (month.length < 2) month = '0' + month;
        if (day.length < 2) day = '0' + day;

        return [year, month, day].join('-');
    }

    // Helper to format time for input[type="time"]
    function formatTime(date) {
        const d = new Date(date);
        let hours = '' + d.getHours();
        let minutes = '' + d.getMinutes();

        if (hours.length < 2) hours = '0' + hours;
        if (minutes.length < 2) minutes = '0' + minutes;

        return [hours, minutes].join(':');
    }

    // Helper to combine date and time inputs into RFC3339 timestamp
    function combineDateTime(dateStr, timeStr) {
        if (!dateStr || !timeStr) return null;
        // Create date object from inputs
        const date = new Date(dateStr + 'T' + timeStr);
        return date.toISOString();
    }

    // --- Create Injection ---

    // Open modal - HANDLES MULTIPLE BUTTONS
    const logInjectionBtns = document.querySelectorAll('[data-action="log-injection"]');
    logInjectionBtns.forEach(btn => {
        btn.addEventListener('click', function () {
            const modal = document.getElementById('log-injection');
            if (modal) {
                // Pre-fill with current date/time
                const now = new Date();
                const dateInput = modal.querySelector('input[name="date"]');
                const timeInput = modal.querySelector('input[name="time"]');

                if (dateInput) dateInput.value = formatDate(now);
                if (timeInput) timeInput.value = formatTime(now);

                modal.showModal();
            }
        });
    });

    // Close modal
    const closeLogInjectionBtns = document.querySelectorAll('[data-action="close-log-injection"]');
    closeLogInjectionBtns.forEach(btn => {
        btn.addEventListener('click', function () {
            const modal = document.getElementById('log-injection');
            if (modal) modal.close();
        });
    });

    // Handle create form submission
    const createForm = document.getElementById('log-injection-form');
    if (createForm) {
        // Range slider update
        const rangeInput = createForm.querySelector('input[type="range"]');
        const rangeOutput = createForm.querySelector('[data-output="pain-level"]');
        if (rangeInput && rangeOutput) {
            rangeInput.addEventListener('input', function () {
                rangeOutput.textContent = this.value;
            });
        }

        createForm.addEventListener('submit', function (e) {
            e.preventDefault();

            const formData = new FormData(e.target);
            const courseId = e.target.getAttribute('data-course-id');
            const btn = e.target.querySelector('button[type=submit]');

            // Get date/time
            const dateStr = formData.get('date');
            const timeStr = formData.get('time');
            const timestamp = combineDateTime(dateStr, timeStr);

            const data = {
                course_id: parseInt(courseId),
                side: formData.get('side'),
                pain_level: parseInt(formData.get('pain_level')),
                notes: formData.get('notes'),
                timestamp: timestamp
            };

            btn.disabled = true;
            btn.setAttribute('aria-busy', 'true');

            fetch('/api/injections', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-CSRF-Token': getCSRFToken()
                },
                body: JSON.stringify(data)
            })
                .then(response => {
                    btn.disabled = false;
                    btn.removeAttribute('aria-busy');
                    if (response.ok) {
                        window.location.reload();
                    } else {
                        return response.text().then(text => {
                            console.error('Error:', text);
                            alert('Error: ' + text);
                        });
                    }
                })
                .catch(error => {
                    btn.disabled = false;
                    btn.removeAttribute('aria-busy');
                    console.error('Error:', error.message);
                    alert('Error: ' + error.message);
                });
        });
    }

    // --- Edit Injection ---

    // Open edit modals
    document.querySelectorAll('[data-action="edit-injection"]').forEach(btn => {
        btn.addEventListener('click', function () {
            const id = this.getAttribute('data-id');
            const modal = document.getElementById('edit-injection-' + id);

            // Date/Time are pre-filled by backend template logic or we could parse them here
            // Parsing existing timestamp from data attribute would be robust

            if (modal) modal.showModal();
        });
    });

    // Close edit modals
    document.querySelectorAll('[data-action="close-edit-injection"]').forEach(btn => {
        btn.addEventListener('click', function () {
            const id = this.getAttribute('data-id');
            const modal = document.getElementById('edit-injection-' + id);
            if (modal) modal.close();
        });
    });

    // Handle edit forms
    document.querySelectorAll('[data-form="edit-injection"]').forEach(form => {
        const id = form.getAttribute('data-id');

        // Range slider update
        const rangeInput = form.querySelector('input[type="range"]');
        const rangeOutput = form.querySelector('[data-output="pain-level"]');
        if (rangeInput && rangeOutput) {
            rangeInput.addEventListener('input', function () {
                rangeOutput.textContent = this.value;
            });
        }

        form.addEventListener('submit', function (e) {
            e.preventDefault();

            const formData = new FormData(e.target);
            const btn = e.target.querySelector('button[type=submit]');

            // Get date/time
            const dateStr = formData.get('date');
            const timeStr = formData.get('time');
            const timestamp = combineDateTime(dateStr, timeStr);

            const data = {
                side: formData.get('side-' + id), // Radio buttons have unique names
                pain_level: parseInt(formData.get('pain_level')),
                notes: formData.get('notes'),
                timestamp: timestamp
            };

            btn.disabled = true;
            btn.setAttribute('aria-busy', 'true');

            fetch('/api/injections/' + id, {
                method: 'PUT',
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
                            btn.disabled = false;
                            btn.removeAttribute('aria-busy');
                            console.error('Error:', text);
                            alert('Error: ' + text);
                        });
                    }
                })
                .catch(error => {
                    btn.disabled = false;
                    btn.removeAttribute('aria-busy');
                    console.error('Error:', error.message);
                    alert('Error: ' + error.message);
                });
        });
    });

    // --- Delete Injection ---

    let currentDeleteId = null;
    const deleteConfirmModal = document.getElementById('delete-injection-confirm');
    const deleteInfoSpan = document.getElementById('delete-injection-info');

    // Open delete confirmation
    document.querySelectorAll('[data-action="delete-injection"]').forEach(btn => {
        btn.addEventListener('click', function () {
            currentDeleteId = this.getAttribute('data-id');
            const side = this.getAttribute('data-side');
            const date = this.getAttribute('data-date');

            if (deleteInfoSpan) {
                deleteInfoSpan.textContent = side + ' side on ' + date;
            }
            if (deleteConfirmModal) {
                deleteConfirmModal.showModal();
            }
        });
    });

    // Close delete confirmation
    document.querySelectorAll('[data-action="close-delete-confirm"]').forEach(btn => {
        btn.addEventListener('click', function () {
            if (deleteConfirmModal) deleteConfirmModal.close();
            currentDeleteId = null;
        });
    });

    // Confirm delete
    const confirmDeleteBtn = document.querySelector('[data-action="confirm-delete"]');
    if (confirmDeleteBtn) {
        confirmDeleteBtn.addEventListener('click', function () {
            if (!currentDeleteId) return;

            // Disable button
            this.disabled = true;
            this.setAttribute('aria-busy', 'true');

            fetch('/api/injections/' + currentDeleteId, {
                method: 'DELETE',
                headers: { 'X-CSRF-Token': getCSRFToken() }
            }).then(response => {
                if (response.ok) {
                    window.location.reload();
                } else {
                    this.disabled = false;
                    this.removeAttribute('aria-busy');
                    console.error('Error deleting injection');
                    alert('Error deleting injection');
                }
            }).catch(error => {
                this.disabled = false;
                this.removeAttribute('aria-busy');
                console.error('Error:', error);
                alert('Error: ' + error.message);
            });
        });
    }
});
