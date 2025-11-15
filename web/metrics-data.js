// metrics.js - Handle the live metrics table display

// Fetch and display current metric values
function updateMetricsTable() {
    fetch('/api/metrics-data')
        .then(r => r.json())
        .then(data => {
            // Update system-info
            if (data.system) {
                document.getElementById('output-type').textContent = data.system.outputType;

                if (data.system.outputType === 'MIDI') {
                    document.getElementById('midi-details').style.display = 'block';
                    document.getElementById('midi-port').textContent = data.system.midiPort || '-';
                    document.getElementById('midi-channel').textContent = data.system.midiChannel ?? '-';
                    document.getElementById('midi-root').textContent = data.system.midiRoot ?? '-';
                    document.getElementById('midi-scale').textContent = data.system.midiScale;
                    document.getElementById('midi-notes').textContent = data.system.midiNotes;
                } else {
                    document.getElementById('midi-details').style.display = 'none';
                }
            }

            const tbody = document.getElementById('metrics-tbody');
            tbody.innerHTML = '';

            data.metrics.forEach(metric => {
                const row = tbody.insertRow();

                // Endpoint
                row.insertCell().textContent = metric.endpoint;

                // Metric name
                const metricCell = row.insertCell();
                metricCell.textContent = metric.metric;
                metricCell.style.fontFamily = 'monospace';

                // Current value
                const currentCell = row.insertCell();
                currentCell.textContent = metric.currentVal.toLocaleString();
                currentCell.style.textAlign = 'right';
                currentCell.style.fontFamily = 'monospace';
                currentCell.style.color = metric.isAccent ? '#00fce7' : '#aaa';

                // Max value
                const maxCell = row.insertCell();
                maxCell.textContent = metric.maxVal.toLocaleString();
                maxCell.style.textAlign = 'right';
                maxCell.style.fontFamily = 'monospace';

                // Percentage
                const pctCell = row.insertCell();
                pctCell.textContent = metric.percentUsed.toFixed(1) + '%';
                pctCell.style.textAlign = 'right';
                pctCell.style.fontFamily = 'monospace';

                // Color code the percentage
                if (metric.percentUsed >= 100) {
                    pctCell.style.color = '#ff7f00'; // Orange - accent triggered
                } else if (metric.percentUsed >= 80) {
                    pctCell.style.color = '#ffcc00'; // Yellow - getting close
                } else {
                    pctCell.style.color = '#5fa73b'; // Green - normal
                }

                // Status indicator
                const statusCell = row.insertCell();
                statusCell.style.textAlign = 'center';
                if (metric.isAccent) {
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
            });
        })
        .catch(err => console.error('Failed to fetch metrics:', err));
}

// Update table every 2 seconds
setInterval(updateMetricsTable, 2000);
updateMetricsTable(); // Initial load