/**
 * Inventory Management JavaScript
 * Handles inventory CRUD operations and settings with proper event listeners for CSP compliance
 */

document.addEventListener('DOMContentLoaded', function () {
    // Get CSRF token from meta tag
    function getCSRFToken() {
        const metaTag = document.querySelector('meta[name="csrf-token"]');
        return metaTag ? metaTag.getAttribute('content') : '';
    }

    // --- Add Inventory Form ---
    const addInventoryForm = document.getElementById('add-inventory-form');
    if (addInventoryForm) {
        const itemTypeSelect = document.getElementById('add-item-type');
        const vialSizeContainer = document.getElementById('add-vial-size-container');
        const amountLabel = document.getElementById('add-amount-label');
        const amountInput = document.getElementById('add-amount-input');
        const lowStockInput = document.getElementById('add-low-stock-input');

        // Handle item type change
        if (itemTypeSelect) {
            itemTypeSelect.addEventListener('change', function () {
                const isProgesterone = this.value === 'progesterone';

                // Toggle vial size field
                if (vialSizeContainer) {
                    vialSizeContainer.style.display = isProgesterone ? 'block' : 'none';
                    const input = vialSizeContainer.querySelector('input');
                    if (input) input.required = isProgesterone;
                }

                // Update amount label and step
                if (amountLabel) amountLabel.textContent = isProgesterone ? 'Number of Vials' : 'Quantity';
                if (amountInput) amountInput.step = isProgesterone ? '1' : '1';

                // Update low stock step
                if (lowStockInput) lowStockInput.step = isProgesterone ? '0.1' : '1';
            });
            // Trigger change initially
            itemTypeSelect.dispatchEvent(new Event('change'));
        }

        // Handle submission
        addInventoryForm.addEventListener('submit', function (e) {
            e.preventDefault();
            const btn = this.querySelector('button[type=submit]');
            btn.disabled = true;
            btn.textContent = 'Adding...';

            const formData = new FormData(this);
            const itemType = formData.get('item_type');
            const amount = parseFloat(formData.get('amount'));
            const vialSize = formData.get('vial_size') ? parseFloat(formData.get('vial_size')) : 0;

            let changeAmount;
            if (itemType === 'progesterone') {
                changeAmount = vialSize * amount;
            } else {
                changeAmount = amount;
            }

            const data = {
                change_amount: changeAmount,
                reason: 'restock', // Default for add form
                notes: 'Added inventory'
            };

            const lotNumber = formData.get('lot_number');
            const expirationDate = formData.get('expiration_date');
            const lowStockThreshold = formData.get('low_stock_threshold');

            if (lotNumber) data.lot_number = lotNumber;
            if (expirationDate) data.expiration_date = expirationDate;
            if (lowStockThreshold) data.low_stock_threshold = parseFloat(lowStockThreshold);

            fetch('/api/inventory/' + itemType + '/adjust', {
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
                            btn.disabled = false;
                            btn.textContent = 'Add to Inventory';
                            console.error('Error:', text);
                            alert('Error: ' + text);
                        });
                    }
                })
                .catch(error => {
                    btn.disabled = false;
                    btn.textContent = 'Add to Inventory';
                    console.error('Error:', error.message);
                    alert('Error: ' + error.message);
                });
        });
    }

    // --- Adjust Inventory Toggles & Forms ---

    // Toggle adjust forms
    document.querySelectorAll('[data-action="toggle-adjust"]').forEach(btn => {
        btn.addEventListener('click', function () {
            const itemType = this.getAttribute('data-item-type');
            const container = document.getElementById('adjust-container-' + itemType);
            const isVisible = container.style.display !== 'none';

            container.style.display = isVisible ? 'none' : 'block';
            this.textContent = isVisible ? 'Adjust' : 'Cancel';
        });
    });

    // Handle adjust forms
    document.querySelectorAll('[data-form="adjust-inventory"]').forEach(form => {
        const itemType = form.getAttribute('data-item-type');
        const reasonSelect = form.querySelector('select[name="reason"]');
        const restockFields = document.getElementById('restock-fields-' + itemType);

        // Handle reason change to show/hide restock fields
        if (reasonSelect) {
            reasonSelect.addEventListener('change', function () {
                if (restockFields) {
                    restockFields.style.display = this.value === 'restock' ? 'block' : 'none';
                }
            });
        }

        form.addEventListener('submit', function (e) {
            e.preventDefault();
            const btn = this.querySelector('button[type=submit]');
            btn.disabled = true;
            btn.setAttribute('aria-busy', 'true');

            const formData = new FormData(this);
            const adjustAmount = parseFloat(formData.get('amount'));
            const reason = formData.get('reason');

            const data = {
                change_amount: adjustAmount,
                reason: reason,
                notes: formData.get('notes') || null
            };

            if (reason === 'restock') {
                const lotNum = formData.get('lot_number');
                const expDate = formData.get('expiration_date');
                if (lotNum) data.lot_number = lotNum;
                if (expDate) data.expiration_date = expDate;
            }

            const lowStockThreshold = formData.get('low_stock_threshold');
            if (lowStockThreshold && parseFloat(lowStockThreshold) > 0) {
                data.low_stock_threshold = parseFloat(lowStockThreshold);
            }

            fetch('/api/inventory/' + itemType + '/adjust', {
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
                            console.error('Error:', text);
                            btn.disabled = false;
                            btn.removeAttribute('aria-busy');
                            alert('Error: ' + text);
                        });
                    }
                })
                .catch(error => {
                    console.error('Error:', error.message);
                    btn.disabled = false;
                    btn.removeAttribute('aria-busy');
                    alert('Error: ' + error.message);
                });
        });
    });

    // --- Settings Form ---
    const settingsForm = document.getElementById('inventory-settings-form');
    if (settingsForm) {
        settingsForm.addEventListener('submit', function (e) {
            e.preventDefault();
            const btn = this.querySelector('button[type=submit]');
            btn.disabled = true;
            btn.textContent = 'Saving...';
            const feedback = document.getElementById('settings-feedback');

            const formData = new FormData(this);
            const progPerInj = parseFloat(formData.get('progesterone_per_injection'));
            const autoDeduct = formData.get('auto_deduct') === 'on';

            fetch('/api/inventory/settings', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-CSRF-Token': getCSRFToken()
                },
                body: JSON.stringify({
                    progesterone_per_injection: progPerInj,
                    auto_deduct: autoDeduct
                })
            })
                .then(response => {
                    if (response.ok) {
                        feedback.innerHTML = '<div class="alert-success">Settings saved successfully!</div>';
                        setTimeout(() => { feedback.innerHTML = ''; }, 3000);
                    } else {
                        return response.text().then(text => {
                            feedback.innerHTML = '<div class="alert-danger">Error: ' + text + '</div>';
                        });
                    }
                    btn.disabled = false;
                    btn.textContent = 'Save Settings';
                })
                .catch(error => {
                    feedback.innerHTML = '<div class="alert-danger">Error: ' + error.message + '</div>';
                    btn.disabled = false;
                    btn.textContent = 'Save Settings';
                });
        });
    }

    // --- History Navigation ---
    document.querySelectorAll('[data-action="view-history"]').forEach(btn => {
        btn.addEventListener('click', function () {
            const itemType = this.getAttribute('data-item-type');
            window.location.href = '/inventory/' + itemType + '/history';
        });
    });
});
