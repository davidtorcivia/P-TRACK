/**
 * Course Management JavaScript
 * Handles course CRUD operations with proper event listeners for CSP compliance
 */

document.addEventListener('DOMContentLoaded', function () {
    // Get CSRF token from meta tag
    function getCSRFToken() {
        const metaTag = document.querySelector('meta[name="csrf-token"]');
        return metaTag ? metaTag.getAttribute('content') : '';
    }

    // Show new course modal
    const createCourseButtons = document.querySelectorAll('[data-action="create-course"]');
    createCourseButtons.forEach(btn => {
        btn.addEventListener('click', function (e) {
            e.preventDefault();
            const modal = document.getElementById('new-course');
            if (modal) modal.showModal();
        });
    });

    // New course form submission
    const newCourseForm = document.getElementById('new-course-form');
    if (newCourseForm) {
        newCourseForm.addEventListener('submit', function (e) {
            e.preventDefault();
            const formData = new FormData(e.target);
            const data = {
                name: formData.get('name'),
                start_date: formData.get('start_date'),
                expected_end_date: formData.get('expected_end_date') || null,
                notes: formData.get('notes') || null
            };
            const btn = e.target.querySelector('button[type=submit]');
            btn.disabled = true;
            btn.setAttribute('aria-busy', 'true');

            fetch('/api/courses', {
                method: 'POST',
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
                            alert('Error: ' + text);
                            btn.disabled = false;
                            btn.removeAttribute('aria-busy');
                        });
                    }
                })
                .catch(error => {
                    alert('Error: ' + error.message);
                    btn.disabled = false;
                    btn.removeAttribute('aria-busy');
                });
        });
    }

    // Close new course modal
    const closeNewCourseButtons = document.querySelectorAll('[data-action="close-new-course"]');
    closeNewCourseButtons.forEach(btn => {
        btn.addEventListener('click', function () {
            const modal = document.getElementById('new-course');
            if (modal) modal.close();
        });
    });

    // Edit course modal buttons
    document.querySelectorAll('[data-action="edit-course"]').forEach(btn => {
        btn.addEventListener('click', function () {
            const courseId = this.getAttribute('data-course-id');
            const modal = document.getElementById('edit-course-' + courseId);
            if (modal) modal.showModal();
        });
    });

    // Close edit course modal buttons
    document.querySelectorAll('[data-action="close-edit-course"]').forEach(btn => {
        btn.addEventListener('click', function () {
            const courseId = this.getAttribute('data-course-id');
            const modal = document.getElementById('edit-course-' + courseId);
            if (modal) modal.close();
        });
    });

    // Edit course form submissions
    document.querySelectorAll('[data-form="edit-course"]').forEach(form => {
        form.addEventListener('submit', function (e) {
            e.preventDefault();
            const courseId = this.getAttribute('data-course-id');
            const formData = new FormData(e.target);
            const data = {
                name: formData.get('name'),
                start_date: formData.get('start_date'),
                expected_end_date: formData.get('expected_end_date') || null,
                actual_end_date: formData.get('actual_end_date') || null,
                notes: formData.get('notes') || null
            };
            const btn = e.target.querySelector('button[type=submit]');
            btn.disabled = true;
            btn.setAttribute('aria-busy', 'true');

            fetch('/api/courses/' + courseId, {
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
                            alert('Error: ' + text);
                            btn.disabled = false;
                            btn.removeAttribute('aria-busy');
                        });
                    }
                })
                .catch(error => {
                    alert('Error: ' + error.message);
                    btn.disabled = false;
                    btn.removeAttribute('aria-busy');
                });
        });
    });

    // Close course buttons
    document.querySelectorAll('[data-action="close-course"]').forEach(btn => {
        btn.addEventListener('click', function () {
            const courseId = this.getAttribute('data-course-id');
            if (confirm('Close this course?')) {
                fetch('/api/courses/' + courseId + '/close', {
                    method: 'POST',
                    headers: { 'X-CSRF-Token': getCSRFToken() }
                })
                    .then(response => {
                        if (response.ok) {
                            window.location.reload();
                        } else {
                            response.text().then(text => console.error('Error:', text));
                            window.location.reload();
                        }
                    });
            }
        });
    });

    // Reactivate course buttons
    document.querySelectorAll('[data-action="activate-course"]').forEach(btn => {
        btn.addEventListener('click', function () {
            const courseId = this.getAttribute('data-course-id');
            fetch('/api/courses/' + courseId + '/activate', {
                method: 'POST',
                headers: { 'X-CSRF-Token': getCSRFToken() }
            }).then(response => {
                if (response.ok) {
                    window.location.reload();
                } else {
                    console.error('Error activating course');
                    window.location.reload();
                }
            });
        });
    });

    // Delete course buttons
    document.querySelectorAll('[data-action="delete-course"]').forEach(btn => {
        btn.addEventListener('click', function () {
            const courseId = this.getAttribute('data-course-id');
            const courseName = this.getAttribute('data-course-name');
            if (confirm('Delete ' + courseName + '? This will delete all associated data.')) {
                fetch('/api/courses/' + courseId, {
                    method: 'DELETE',
                    headers: { 'X-CSRF-Token': getCSRFToken() }
                }).then(response => {
                    if (response.ok) {
                        window.location.reload();
                    } else {
                        console.error('Error deleting course');
                        window.location.reload();
                    }
                });
            }
        });
    });
});
