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

// MIDI only queue reporting
let midiPollInterval = null;

// init queue monitoring on page load
async function initQueueMonitor() {
    try {
        const response = await fetch('/api/plugin/type', {method: 'POST'});
        const data = await response.json();

        // Only show queue panel for MIDI output
        if (data === 'MIDI') {
            document.getElementById('queuePanel').style.display = 'block';
            midiQueuePoller();
        }
    } catch (error) {
        console.error('Failed to check output type:', error);
    }
}

function midiQueuePoller() {
    midiQueryRange();
    midiPollInterval = setInterval(midiQueryRange, 500);
}

function midiStopPoller() {
    if (midiPollInterval) {
        clearInterval(midiPollInterval);
        midiPollInterval = null;
    }
}

async function midiQueryRange() {
    try {
        const response = await fetch('/api/plugin/queryrange', { method: 'POST' });
        if (!response.ok) {
            midiStopPoller();
            return;
        }

        const data = await response.json();

        // Update the display
        document.getElementById('noteDepth').textContent = data.noteDepth;
        document.getElementById('noteWindow').textContent =
            (data.noteWindow / 1000000000).toFixed(2); // nanoseconds to seconds

        if (data.noteDepth > 0) {
            document.getElementById('noteOldest').textContent =
                new Date(data.noteOldest).toLocaleTimeString();
            document.getElementById('noteNewest').textContent =
                new Date(data.noteNewest).toLocaleTimeString();

            // Update bar graph (assume max queue of 100 notes)
            const percentage = Math.min((data.noteDepth / 100) * 100, 100);
            document.getElementById('queueBarFill').style.width = percentage + '%';

            // Update concurrency bar graph (assume upper bound of 100)
            const percentGoRoutine = Math.min((data.activeRoutines / 100) * 100, 100);
            document.getElementById('routineBarFill').style.width = percentGoRoutine + '%';
        } else {
            document.getElementById('noteOldest').textContent = '--';
            document.getElementById('noteNewest').textContent = '--';
            document.getElementById('queueBarFill').style.width = '0%';
            document.getElementById('routineBarFill').style.width = '0%';
        }

        // These should always execute
        document.getElementById('noteGrouperSize').textContent = data.noteGrouperSize;
        document.getElementById('activeRoutines').textContent = data.activeRoutines;
    } catch (error) {
        console.error('Failed to query queue:', error);
        midiStopPoller();
    }
}

// Initialize on page load
document.addEventListener('DOMContentLoaded', initQueueMonitor);

// Clean up on page unload
window.addEventListener('beforeunload', midiStopPoller);
