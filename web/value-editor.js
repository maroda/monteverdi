// Initialize on page load
document.addEventListener('DOMContentLoaded', () => {
   loadCurrentConfig();

   // Button handlers
    document.getElementById('validate-button').addEventListener('click', validateJSON);
    document.getElementById('submit-button').addEventListener('click', submitConfig);
})

// Fetch current config
async function loadCurrentConfig() {
    try {
        const response = await fetch('/conf');
        if (!response.ok) {
            throw new Error(`HTTP error: ${response.status}`);
        }
        const config = await response.json();

        // Pretty-print JSON into textarea
        document.getElementById('config-textarea').value = JSON.stringify(config, null, 2);
    } catch (error) {
        showStatus('Failed to load config ' + error.message, 'error');
    }
}

// Display status messages
function showStatus(message, type) {
    const statusDiv = document.getElementById('status-message');
    statusDiv.textContent = message;
    statusDiv.className = type;
    statusDiv.style.display = 'block';

    // Auto-hide after 5 seconds for success messages
    if (type === 'success') {
        setTimeout(() => {
            statusDiv.style.display = 'none';
        }, 5000);
    }
}

// Validate JSON before submission
function validateJSON() {
    const textarea = document.getElementById('config-textarea');
    const jsonText = textarea.value;

    try {
        JSON.parse(jsonText);
        showStatus('Valid JSON! ✓', 'success');
        return true;
    } catch (error) {
        showStatus('Invalid JSON: ' + error.message, 'error');
        return false;
    }
}

// Submit new config
async function submitConfig() {
    const textarea = document.getElementById('config-textarea');
    const jsonText = textarea.value;

    // Validate first
    try {
        JSON.parse(jsonText);
    } catch (error) {
        showStatus('Invalid JSON: ' + error.message, 'error');
        return;
    }

    // POST to /conf endpoint
    try {
        const response = await fetch('/conf', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: jsonText
        });

        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        const result = await response.json();
        showStatus('Configuration updated successfully! ✓', 'success');
    } catch (error) {
        showStatus('Failed to update config ' + error.message, 'error');
    }
}

// Value picker functions
// Fetch all metrics and populate dropdown
async function loadMetricsList() {
    try {
        const response = await fetch('/api/metrics-data');
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const metrics = await response.json();

        const select = document.getElementById('metric-select');

        // Build unique list of endpoint:metric combinations
        metrics.forEach(m => {
            const option = document.createElement('option');
            option.value = `${m.endpoint}:${m.metric}`;
            option.textContent = `${m.endpoint} → ${m.metric}`;
            select.appendChild(option);
        });

    } catch (error) {
        console.error('Failed to load metrics list:', error);
    }
}

// Display selected metric in table
async function displaySelectedMetric() {
    const select = document.getElementById('metric-select');
    const selectedValue = select.value;

    if (!selectedValue) {
        document.getElementById('selected-metric-row').style.display = 'none';
        return;
    }

    const [endpoint, metric] = selectedValue.split(':');

    try {
        const response = await fetch('/api/metrics-data');
        const allMetrics = await response.json();

        // Find the matching metric
        const metricData = allMetrics.find(m =>
            m.endpoint === endpoint && m.metric === metric
        );

        if (!metricData) return;

        // Show the row
        document.getElementById('selected-metric-row').style.display = 'block';

        // Populate table (same logic as metrics-data.js)
        const tbody = document.getElementById('selected-metric-tbody');
        tbody.innerHTML = '';

        const row = tbody.insertRow();

        // Endpoint
        row.insertCell().textContent = metricData.endpoint;

        // Metric name
        const metricCell = row.insertCell();
        metricCell.textContent = metricData.metric;
        metricCell.style.fontFamily = 'monospace';

        // Current value
        const currentCell = row.insertCell();
        currentCell.textContent = metricData.currentVal.toLocaleString();
        currentCell.style.textAlign = 'right';
        currentCell.style.fontFamily = 'monospace';
        currentCell.style.color = metricData.isAccent ? '#00fce7' : '#aaa';

        // Max value
        const maxCell = row.insertCell();
        maxCell.textContent = metricData.maxVal.toLocaleString();
        maxCell.style.textAlign = 'right';
        maxCell.style.fontFamily = 'monospace';

        // Percentage
        const pctCell = row.insertCell();
        pctCell.textContent = metricData.percentUsed.toFixed(1) + '%';
        pctCell.style.textAlign = 'right';
        pctCell.style.fontFamily = 'monospace';

        // Color code the percentage
        if (metricData.percentUsed >= 100) {
            pctCell.style.color = '#ff7f00';
        } else if (metricData.percentUsed >= 80) {
            pctCell.style.color = '#ffcc00';
        } else {
            pctCell.style.color = '#5fa73b';
        }

        // Status indicator
        const statusCell = row.insertCell();
        statusCell.style.textAlign = 'center';
        if (metricData.isAccent) {
            statusCell.innerHTML = '🔥';
            statusCell.title = 'Accent triggered!';
        } else {
            statusCell.innerHTML = '✓';
            statusCell.style.color = '#5fa73b';
        }

        // Style the row
        row.style.borderBottom = '1px solid #333';
        Array.from(row.cells).forEach(cell => {
            cell.style.padding = '8px';
        });

    } catch (error) {
        console.error('Failed to display metric:', error);
    }
}

// Update the DOMContentLoaded section:
document.addEventListener('DOMContentLoaded', () => {
    loadCurrentConfig();
    loadMetricsList();

    // Attach button handlers
    document.getElementById('validate-button').addEventListener('click', validateJSON);
    document.getElementById('submit-button').addEventListener('click', submitConfig);

    // Attach dropdown handler
    document.getElementById('metric-select').addEventListener('change', displaySelectedMetric);

    // Auto-refresh selected metric every 2 seconds
    setInterval(() => {
        if (document.getElementById('metric-select').value) {
            displaySelectedMetric();
        }
    }, 2000);
});