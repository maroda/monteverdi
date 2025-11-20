// plugins.js - Handle system info and parameter adjustment

// Fetch and display sysinfo
function updateSysInfoTable() {
    fetch('/api/metrics-data')
        .then(r => r.json())
        .then(data => {
            // Update sysinfo
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
        })
        .catch(err => console.error('Failed to update sysinfo table:', err));
}

updateSysInfoTable();

// Provide an interface to run Plugin control commands
async function controlPlugin(control) {
    const feedbackDiv = document.getElementById('pluginResponse');

    try {
        const response = await fetch(`/api/plugin/${control}`, {
            method: 'POST'
        });

        if (response.ok) {
            const data = await response.json();

            // Show success feedback, hide after 3s
            feedbackDiv.className = 'feedback success';
            feedbackDiv.textContent = `✓ ${control.toUpperCase()}: ${JSON.stringify(data)}`;

            return data;
        } else {
            throw new Error(`${response.status}: ${response.statusText}`);
            console.error(`Plugin ${control} failed:`, response.statusText);
        }
    } catch (error) {
        // Show error feedback
        feedbackDiv.className = 'feedback error';
        feedbackDiv.textContent = `✗ Error: ${error.message}`;

        console.error(`Error sending control to plugin: ${error}`);
    }
}

document.getElementById('outputFlush').addEventListener('click', (e) => { controlPlugin('flush'); })
document.getElementById('outputType').addEventListener('click', (e) => { controlPlugin('type'); })
