/**
 * Symptoms History Page JavaScript
 * Handles symptom loading, filtering, editing, and deletion with CSP compliance
 */

// State management
let currentEditSymptom = null;
let currentDeleteSymptom = null;

// Get CSRF token from meta tag
function getCSRFToken() {
    const meta = document.querySelector('meta[name="csrf-token"]');
    return meta ? meta.getAttribute('content') : '';
}

// Escape user-controlled text before inserting into innerHTML to prevent XSS.
function escapeHtml(value) {
    if (value === null || value === undefined) return '';
    return String(value)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');
}

// Render symptoms list HTML
function renderSymptomsList(data, container) {
    if (data.length === 0) {
        container.innerHTML = '<p style="text-align:center;color:var(--color-text-muted);">No symptoms logged yet.</p>';
        return;
    }

    let html = '';
    data.forEach(symptom => {
        const date = new Date(symptom.timestamp);
        const formattedDate = date.toLocaleDateString('en-US', {
            year: 'numeric',
            month: 'short',
            day: 'numeric',
            hour: '2-digit',
            minute: '2-digit'
        });

        html += '<article>';
        html += '<header>' + formattedDate + '</header>';
        html += '<div><strong>Pain Level:</strong> ' + (symptom.pain_level || 'N/A') + '/10</div>';
        html += '<div><strong>Location:</strong> ' + escapeHtml(symptom.pain_location || 'N/A') + '</div>';
        html += '<div><strong>Type:</strong> ' + escapeHtml(symptom.pain_type || 'N/A') + '</div>';

        if (symptom.symptoms) {
            try {
                const symptoms = JSON.parse(symptom.symptoms);
                if (symptoms.length > 0) {
                    html += '<div><strong>Symptoms:</strong> ' + symptoms.map(escapeHtml).join(', ') + '</div>';
                }
            } catch (e) { }
        }

        if (symptom.notes) {
            html += '<div><strong>Notes:</strong> ' + escapeHtml(symptom.notes) + '</div>';
        }

        html += '<footer style="margin-top:1rem;">';
        html += '<div class="grid" style="grid-template-columns: 1fr 1fr;">';
        html += '<button data-action="delete-symptom" data-symptom-id="' + symptom.id + '" class="outline secondary" style="font-size: 0.9rem;">';
        html += 'Delete</button>';
        html += '<button data-action="edit-symptom" data-symptom-id="' + symptom.id + '" class="outline" style="font-size: 0.9rem;">';
        html += 'Edit</button>';
        html += '</div>';
        html += '</footer>';
        html += '</article>';
    });

    container.innerHTML = html;
}

// Apply symptom filters function
function applySymptomFilters() {
    const startDate = document.querySelector('input[x-model="startDate"]').value;
    const endDate = document.querySelector('input[x-model="endDate"]').value;

    let url = '/api/symptoms?limit=100';
    if (startDate && endDate) {
        url += '&start_date=' + startDate + '&end_date=' + endDate;
    }

    fetch(url)
        .then(r => r.json())
        .then(data => {
            const container = document.getElementById('symptoms-list');
            if (data.length === 0) {
                container.innerHTML = '<p style="text-align:center;color:var(--color-text-muted);">No symptoms found for this date range.</p>';
                return;
            }
            renderSymptomsList(data, container);
        })
        .catch(err => {
            alert('Error loading symptoms: ' + err.message);
        });
}

// Load all symptoms on page load
function loadSymptoms() {
    fetch('/api/symptoms?limit=100')
        .then(r => r.json())
        .then(data => {
            const container = document.getElementById('symptoms-list');
            renderSymptomsList(data, container);
        })
        .catch(err => {
            document.getElementById('symptoms-list').innerHTML =
                '<p style="color:red;">Error loading symptoms: ' + err.message + '</p>';
        });
}

// Function to open edit modal
function editSymptom(symptomId) {
    console.log('editSymptom called with ID:', symptomId);

    fetch('/api/symptoms/' + symptomId)
        .then(response => {
            if (!response.ok) {
                throw new Error('Failed to fetch symptom data');
            }
            return response.json();
        })
        .then(symptom => {
            console.log('Symptom data loaded:', symptom);

            // Parse symptoms array if it exists
            let symptomsArray = [];
            if (symptom.symptoms) {
                try {
                    const parsed = JSON.parse(symptom.symptoms);
                    if (Array.isArray(parsed)) {
                        symptomsArray = parsed;
                    }
                } catch (e) {
                    console.warn('Failed to parse symptoms:', e);
                }
            }

            // Store current symptom data
            currentEditSymptom = {
                id: symptomId,
                painLevel: symptom.pain_level || 5,
                painLocation: symptom.pain_location || '',
                painType: symptom.pain_type || '',
                selectedSymptoms: symptomsArray,
                notes: symptom.notes || ''
            };

            // Populate form fields
            const modal = document.getElementById('symptom-edit-modal');
            if (modal) {
                const painLevelInput = modal.querySelector('#edit-pain-level');
                const painLevelDisplay = modal.querySelector('#pain-level-display');
                const painLocationSelect = modal.querySelector('#edit_pain_location');
                const painTypeSelect = modal.querySelector('#edit_pain_type');
                const notesTextarea = modal.querySelector('#edit_notes');

                if (painLevelInput) {
                    painLevelInput.value = currentEditSymptom.painLevel;
                }
                if (painLevelDisplay) {
                    painLevelDisplay.textContent = currentEditSymptom.painLevel;
                }
                if (painLocationSelect) {
                    painLocationSelect.value = currentEditSymptom.painLocation;
                }
                if (painTypeSelect) {
                    painTypeSelect.value = currentEditSymptom.painType;
                }
                if (notesTextarea) {
                    notesTextarea.value = currentEditSymptom.notes;
                }

                // Clear and set symptom checkboxes
                const checkboxes = modal.querySelectorAll('input[type="checkbox"]');
                checkboxes.forEach(checkbox => {
                    checkbox.checked = currentEditSymptom.selectedSymptoms.includes(checkbox.value);
                });

                // Show modal
                modal.style.display = 'flex';
                console.log('Edit modal shown');
            } else {
                console.error('Edit modal not found');
            }
        })
        .catch(error => {
            console.error('Error loading symptom data:', error);
            alert('Error loading symptom data: ' + error.message);
        });
}

// Function to close edit modal
function closeEditModal() {
    const modal = document.getElementById('symptom-edit-modal');
    if (modal) {
        modal.style.display = 'none';
        console.log('Edit modal hidden');
    }
    currentEditSymptom = null;
}

// Function to open delete modal
function deleteSymptom(symptomId, symptomType) {
    console.log('deleteSymptom called with ID:', symptomId, 'type:', symptomType);

    currentDeleteSymptom = {
        id: symptomId,
        type: symptomType
    };

    const modal = document.getElementById('delete-modal');
    if (modal) {
        const typeText = modal.querySelector('#delete-symptom-type');
        if (typeText) {
            typeText.textContent = symptomType;
        }
        modal.style.display = 'flex';
        console.log('Delete modal shown');
    } else {
        console.error('Delete modal not found');
    }
}

// Function to close delete modal
function closeDeleteModal() {
    const modal = document.getElementById('delete-modal');
    if (modal) {
        modal.style.display = 'none';
        console.log('Delete modal hidden');
    }
    currentDeleteSymptom = null;
}

// Function to submit edit form
function submitEditForm(event) {
    event.preventDefault();

    if (!currentEditSymptom) {
        alert('No symptom data available');
        return;
    }

    const form = event.target;
    const submitBtn = form.querySelector('button[type="submit"]');
    const originalText = submitBtn.textContent;

    submitBtn.disabled = true;
    submitBtn.textContent = 'Updating...';

    const painLevel = parseInt(form.querySelector('#edit-pain-level').value);
    const painLocation = form.querySelector('#edit_pain_location').value;
    const painType = form.querySelector('#edit_pain_type').value;
    const notes = form.querySelector('#edit_notes').value;

    const selectedSymptoms = [];
    form.querySelectorAll('input[type="checkbox"]:checked').forEach(checkbox => {
        selectedSymptoms.push(checkbox.value);
    });

    fetch('/api/symptoms/' + currentEditSymptom.id, {
        method: 'PUT',
        headers: {
            'Content-Type': 'application/json',
            'X-CSRF-Token': getCSRFToken()
        },
        body: JSON.stringify({
            pain_level: painLevel,
            pain_location: painLocation || null,
            pain_type: painType || null,
            symptoms: selectedSymptoms.length > 0 ? selectedSymptoms : null,
            notes: notes || null
        })
    })
        .then(response => {
            if (response.ok) {
                closeEditModal();
                window.location.reload();
            } else {
                return response.text().then(text => {
                    alert('Error: ' + text);
                    submitBtn.disabled = false;
                    submitBtn.textContent = originalText;
                });
            }
        })
        .catch(error => {
            console.error('Error updating symptom:', error);
            alert('Error: ' + error.message);
            submitBtn.disabled = false;
            submitBtn.textContent = originalText;
        });
}

// Function to confirm delete
function confirmDelete() {
    if (!currentDeleteSymptom) {
        alert('No symptom data available');
        return;
    }

    const modal = document.getElementById('delete-modal');
    const deleteBtn = modal.querySelector('[data-action="confirm-delete-symptom"]');
    const originalText = deleteBtn.textContent;

    deleteBtn.disabled = true;
    deleteBtn.textContent = 'Deleting...';

    fetch('/api/symptoms/' + currentDeleteSymptom.id, {
        method: 'DELETE',
        headers: {
            'Content-Type': 'application/json',
            'X-CSRF-Token': getCSRFToken()
        }
    })
        .then(response => {
            if (response.ok) {
                closeDeleteModal();
                window.location.reload();
            } else {
                return response.text().then(text => {
                    alert('Error: ' + text);
                    deleteBtn.disabled = false;
                    deleteBtn.textContent = originalText;
                });
            }
        })
        .catch(error => {
            console.error('Error deleting symptom:', error);
            alert('Error: ' + error.message);
            deleteBtn.disabled = false;
            deleteBtn.textContent = originalText;
        });
}

// Initialize page
document.addEventListener('DOMContentLoaded', function () {
    console.log('DOM loaded, setting up event delegation for history page');

    // Load symptoms on page load
    loadSymptoms();

    // Event delegation for all action buttons
    document.body.addEventListener('click', function (e) {
        const button = e.target.closest('[data-action]');
        if (!button) return;

        console.log('History Button clicked:', button);
        const action = button.dataset.action;
        const symptomId = button.dataset.symptomId;

        console.log('History Action:', action, 'Symptom ID:', symptomId);

        if (action === 'edit-symptom') {
            e.preventDefault();
            editSymptom(symptomId);
        } else if (action === 'delete-symptom') {
            e.preventDefault();
            deleteSymptom(symptomId, 'symptom log');
        } else if (action === 'back-to-symptoms') {
            e.preventDefault();
            window.location.href = '/symptoms';
        } else if (action === 'apply-filters') {
            e.preventDefault();
            applySymptomFilters();
        } else if (action === 'close-delete-modal') {
            e.preventDefault();
            closeDeleteModal();
        } else if (action === 'confirm-delete-symptom') {
            e.preventDefault();
            confirmDelete();
        } else if (action === 'close-edit-modal') {
            e.preventDefault();
            closeEditModal();
        }
    });

    // Handle form submit for edit modal
    const editForm = document.getElementById('symptom-edit-form');
    if (editForm) {
        editForm.addEventListener('submit', submitEditForm);
    }

    // Handle range slider update
    const painLevelRange = document.getElementById('edit-pain-level');
    if (painLevelRange) {
        painLevelRange.addEventListener('input', function () {
            document.getElementById('pain-level-display').textContent = this.value;
        });
    }
});
