// index.js - Simple D3.js example with circles

// Set up dimensions: this is the gray #chart area
const width = 500;
const height = 500;

// Create the SVG element
const svg = d3.select("#chart")
    .attr("width", width)
    .attr("height", height);

// Radar setup
const centerX = width / 2;
const centerY = height / 2;

// Initialize
let currentTime = new Date();
let timeRings = [];
let knownMetrics = new Set();
let initialized = false;
let pulseLengthMultiplier = 0.4; // Starting value in the UI
let ringSpacing = 12; // Starting value in the UI
let highlightThreshold = 3; // Starting value in the UI
let r0expScale = false; // Starting value in the UI
let r1expScale = false; // Starting value in the UI
let showRipple = false; // Starting value in the UI
let lastReceivedData = [];
let currentHighlightedPulse = null;
let activePulseTypes = new Set();
let seenPulses = new Set();
let hoverTimeout;

// Update time every second
setInterval(() => {
    currentTime = new Date();
    updateTimeDisplay();
}, 1000);

// WebSocket connection
const ws = new WebSocket('ws://localhost:8090/ws')

ws.onmessage = function(event) {
    const data = JSON.parse(event.data);
    lastReceivedData = data;
    // console.log('Received data, length:', data.length);

    // Discover metrics dynamically
    const oldSize = knownMetrics.size;
    data.forEach(d => {
        if (d.metric) {
            knownMetrics.add(d.metric);
        }
    });

    // Only rebuild rings if we found new metrics
    if (knownMetrics.size > oldSize || !initialized) {
        // console.log('Rebuilding rings, metrics:', Array.from(knownMetrics));
        updateRingStructure();
        initialized = true;
    }

    updatePulsesFromBackend(data);
};

// Retrieve Version for display
fetch('/api/version')
    .then(r => r.json())
    .then(data => {
        d3.select('h1').append('span')
            .style('font-size', '0.5em')
            .style('color', '#888')
            .style('margin-left', '10px')
            .text(`${data.version}`);
    })
    .catch(err => console.log('Version fetch failed:', err));

// HTML Elements
// Exponential Scale Checkboxes
document.getElementById('ring0Scale').addEventListener('change', function(event) {
    r0expScale = event.target.checked;
    if (lastReceivedData.length > 0) {
        updatePulsesFromBackend(lastReceivedData);
    }
});

document.getElementById('ring1Scale').addEventListener('change', function(event) {
    r1expScale = event.target.checked;
    if (lastReceivedData.length > 0) {
        updatePulsesFromBackend(lastReceivedData);
    }
});

// Show Grid Checkbox
document.getElementById('showGrid').addEventListener('change', function(event) {
    if (event.target.checked) {
        drawRadialGrid();
    } else {
        eraseRadialGrid();
    }
})

document.getElementById('showRipple').addEventListener('change', function(event) {
    showRipple = event.target.checked;
})

// Select Threshold Value
document.getElementById('thresholdMinus').addEventListener('click', function() {
    const input = document.getElementById('thresholdInput');
    highlightThreshold = Math.max(3, highlightThreshold - 3);
    input.value = highlightThreshold;

    if (currentHighlightedPulse) {
        highlightRelatedPulses(currentHighlightedPulse);
    }
});

document.getElementById('thresholdPlus').addEventListener('click', function() {
    const input = document.getElementById('thresholdInput');
    highlightThreshold = Math.min(60, highlightThreshold + 3);
    input.value = highlightThreshold;

    if (currentHighlightedPulse) {
        highlightRelatedPulses(currentHighlightedPulse);
    }
});

document.getElementById('thresholdInput').addEventListener('change', function(event) {
    highlightThreshold = Math.max(3, Math.min(60, parseInt(event.target.value)));
    this.value = highlightThreshold; // Clamp the displayed value too

    if (currentHighlightedPulse) {
        highlightRelatedPulses(currentHighlightedPulse);
    }
});

// Control Sliders
document.getElementById('lengthSlider').addEventListener('input', function(event) {
    pulseLengthMultiplier = parseFloat(event.target.value);
    document.getElementById('lengthValue').textContent = pulseLengthMultiplier;

    // Redraw with current data
    if (lastReceivedData.length > 0) {
        updatePulsesFromBackend(lastReceivedData);
    }
});

document.getElementById('spacingSlider').addEventListener('input', function(event) {
    ringSpacing = parseInt(event.target.value);
    document.getElementById('spacingValue').textContent = ringSpacing;

    // Rebuild rings with new spacing
    updateRingStructure();

    // Redraw pulses with current data
    if (lastReceivedData.length > 0) {
        updatePulsesFromBackend(lastReceivedData);
    }
});

// Show intro tooltip on load, hide after delay
window.addEventListener('DOMContentLoaded', function() {
    const intro = document.getElementById('introTooltip');

    if (intro) {
        // Trigger fade-in immediately
        setTimeout(() => intro.classList.add('visible'), 50);

        // Start fade-out after 3 seconds
        setTimeout(() => {
            intro.classList.remove('visible');
            intro.classList.add('hidden');
            // Remove from DOM after fade completes
            setTimeout(() => intro.remove(), 3000);
        }, 3000);
    }
});

function updatePulsesFromBackend(backendData) {
    // Filter data
    const filteredData = backendData.filter(d => {
        const dimension = d.dimension || 1;
        const ring = d.ring || 0;
        return (dimension === 1 && ring === 0) || (dimension === 2 && ring === 1);
    });

    // Track new amphibrachs for animation
    const newAmphibrachs = filteredData.filter(d =>
        d.dimension === 2 &&
        d.type === 'amphibrach' &&
        !seenPulses.has(`${d.metric}-${d.startTime}-${d.type}`)
    );

    // Animate transitions for new amphibrachs
    newAmphibrachs.forEach(d2Pulse => {
        animateD1ToD2Transition(null, d2Pulse);
    });

    // Track NEW pulses for blinking
    activePulseTypes.clear();
    filteredData.forEach(d => {
        if (d.type) {
            // Create unique key for this pulse
            const pulseKey = `${d.metric}-${d.startTime}-${d.type}`;

            // If we havent' seen this pulse, it's NEW, blink!
            if (!seenPulses.has(pulseKey)) {
                seenPulses.add(pulseKey);
                activePulseTypes.add(d.type);
            }
        }
    });

    // Update pulse indicator lights
    updatePulseIndicators();

    // Simple data join with good keys
    const pulses = svg.selectAll('.pulse')
        .data(filteredData, d => `${d.metric}-${d.startTime}-${d.dimension}`);

    // Remove dots that are no longer in data
    pulses.exit().remove();

    // Add new dots
    pulses.enter()
        .append('ellipse')
        .attr('class', d => `pulse pulse-${d.type}`)
        .attr('cx', d => getPulsePosition(d.ring, d.angle, d.metric).x)
        .attr('cy', d => getPulsePosition(d.ring, d.angle, d.metric).y)
        .attr('rx', d => calculatePulseLength(d)) // horizontal radius (length)
        .attr('ry', d => Math.pow(d.intensity || 0.5, 0.7) * 3 + 1) // vertical radius (height)
        .attr('transform', d => {
            const pos = getPulsePosition(d.ring, d.angle, d.metric);
            const tAngle = transformAngle(d.angle, d.ring);
            return `rotate(${tAngle} ${pos.x} ${pos.y})`;
        })
        .style('opacity', d => 0.2 + (d.intensity || 0.5) * 0.4)
        .style('cursor', 'pointer')
        .on('click', function(event, d) { showPulseMeta(d); })
        .on('mouseover', function(event, d) {
            highlightRelatedPulses(d);
        })
        .on('mouseout', function(event, d) {
            clearHighlights();
        });

    // Update positions for existing dots
    pulses
        .attr('cx', d => getPulsePosition(d.ring, d.angle, d.metric).x)
        .attr('cy', d => getPulsePosition(d.ring, d.angle, d.metric).y)
        .attr('transform', d => {
            const pos = getPulsePosition(d.ring, d.angle, d.metric);
            const tAngle = transformAngle(d.angle, d.ring);
            return `rotate(${tAngle} ${pos.x} ${pos.y})`;
        });
}

function updatePulseIndicators() {
    // console.log('Active pulse types:', Array.from(activePulseTypes)); // Debug line

    const pulseMap = {
        'iamb': 'iambPulse',
        'trochee': 'trocheePulse',
        'amphibrach': 'amphibrachPulse',
        'anapest': 'anapestPulse',
        'dactyl': 'dactylPulse'
    };

    // Flash indicators for active pulses
    activePulseTypes.forEach(pulseType => {
        const elementId = pulseMap[pulseType];
        const element = document.getElementById(elementId);

        if (element && !element.classList.contains('active')) {
            // Add active class
            element.classList.add('active');

            // Remove after a short duration (ms)
            setTimeout(() => {
                element.classList.remove('active');
            }, 200);
        }
    });
}

function highlightRelatedPulses(targetPulse) {
    const angleThreshold = 15; // degrees - pulses within this range

    svg.selectAll('.pulse')
        .style('opacity', function(d) {
            // Same ring?
            if (d.ring !== targetPulse.ring) return 0.1; // Dim other rings

            // Calculate angle distance (accounting for wrap-around)
            const transformedTarget = transformAngle(targetPulse.angle, targetPulse.ring);
            const transformedThis = transformAngle(d.angle, d.ring);

            let angleDiff = Math.abs(transformedTarget - transformedThis);
            if (angleDiff > 180) angleDiff = 360 - angleDiff; // Handle wraparound

            // Close in time?
            if (angleDiff < highlightThreshold) {
                return 0.9; // Highlight
            }
            return 0.2; // Dim
        })
        .style('stroke', function(d) {
            if (d.ring !== targetPulse.ring) return 'none';

            const transformedTarget = transformAngle(targetPulse.angle, targetPulse.ring);
            const transformedThis = transformAngle(d.angle, d.ring);
            let angleDiff = Math.abs(transformedTarget - transformedThis);
            if (angleDiff > 180) angleDiff = 360 - angleDiff;

            return angleDiff < highlightThreshold ? 'rgb(0,252,231)' : 'none';
        })
        .style('stroke-width', 1);
}

function clearHighlights() {
    currentHighlightedPulse = null;
    svg.selectAll('.pulse')
        .style('opacity', d => 0.2 + (d.intensity || 0.5) * 0.4)
        .style('stroke', 'none');
}

function calculatePulseLength(d) {
    if (!d.duration) return 4; // default small size

    const durationSeconds = d.duration / 1000000000; // Convert from nanoseconds

    // Get the ring data to find the radius
    const ringData = timeRings.find(r => r.ring === d.ring && r.metric === d.metric);
    if (!ringData) return 4;

    const radius = ringData.radius;
    const circumference = 2 * Math.PI * radius;

    // Calculate what fraction of the ring this duration represents
    const maxDuration = d.ring === 0 ? 60 : (d.ring === 1 ? 600 : 3600); // seconds for full ring
    const durationFraction = Math.min(durationSeconds / maxDuration, 0.02); // Cap at 2% of ring
    // const durationFraction = Math.min(durationSeconds / maxDuration, 0.2); // Cap at 20% of ring

    // Convert to actual arc length on the ring
    const arcLength = durationFraction * circumference;

    // Scale down a bit so pulses don't touch (multiply by 0.8)
    // return arcLength * 0.4;
    return arcLength * pulseLengthMultiplier;
}

function updateRingStructure() {
    const metrics = Array.from(knownMetrics).sort();
    // console.log('Building rings for metrics:', metrics);

    if (metrics.length === 0) {
        // console.log('No metrics found, skipping ring creation');
        return;
    }

    timeRings = [];
    const baseRadius = 20;
    // const ringSpacing = 20;

    // Ring 0 (inner) - one sub-ring per metric
    metrics.forEach((metric, index) => {
        timeRings.push({
            radius: baseRadius + (index * ringSpacing),
            label: `${metric.substring(0, 20)}... (60s)`,
            ring: 0,
            metric: metric,
            metricIndex: index
        });
    });

    // Ring 1 (middle) - one sub-ring per metric
    metrics.forEach((metric, index) => {
        timeRings.push({
            radius: baseRadius + (metrics.length * ringSpacing) + 20 + (index * ringSpacing),
            label: `${metric.substring(0, 20)}... (10m)`,
            ring: 1,
            metric: metric,
            metricIndex: index
        });
    });

    redrawRings();
    // console.log(`Created ${timeRings.length} rings`);
}

function getPulsePosition(ring, angle, metric) {
    const ringData = timeRings.find(r => r.ring === ring && r.metric === metric);

    if (!ringData) {
        console.log(`No ring found for ring:${ring}, metric:${metric}`);
        return { x: centerX, y: centerY }; // Fallback to center
    }

    const tAngle = transformAngle(angle, ring);
    const radius = ringData.radius;
    const radians = (tAngle - 90) * (Math.PI / 180);
    return {
        x: centerX + Math.cos(radians) * radius,
        y: centerY + Math.sin(radians) * radius
    };
}

function redrawRings() {
    // Remove existing rings
    svg.selectAll('.time-ring').remove();
    svg.selectAll('.ring-label').remove();
    svg.selectAll('.hover-label').remove();

    // Draw new rings with hover
    svg.selectAll('.time-ring')
        .data(timeRings)
        .enter()
        .append('circle')
        .attr('class', 'time-ring')
        .attr('cx', centerX)
        .attr('cy', centerY)
        .attr('r', d => d.radius)
        .style('fill', 'none')
        .style('stroke', '#555')
        .style('stroke-width', 1)
        .style('cursor', 'pointer')
        .on('mouseover', function(event, d) {
            // Find a pulse on this ring/metric to get endpoint info
            const matchingPulse = lastReceivedData.find(p =>
                p.ring === d.ring && p.metric === d.metric
            );
            const pulseEP = matchingPulse ? matchingPulse.endpoint : 'Unknown';
            showHoverLabel(d.metric, pulseEP);
            d3.select(this).style('stroke', '#888').style('stroke-width', 2);
        })
        .on('mouseout', function(event, d) {
            hideHoverLabel();
            d3.select(this).style('stroke', '#555').style('stroke-width', 1);
        });
}

function transformAngle(angle, ring) {
    // Decide if we apply exponential based on ring and toggle
    const useExp = (ring === 0 && r0expScale) || (ring === 1 && r1expScale);
    if (!useExp) return angle;

    // Convert angle to normalized time (0-1)
    let normalized = (270 - angle) / 360.0;
    while (normalized < 0) normalized += 1.0;
    while (normalized >= 1.0) normalized -= 1.0;

    // Apply exponential transform
    let transformed = Math.pow(normalized, 2);

    // Convert back to angle
    let newAngle = 270 - (transformed * 360.0);
    while (newAngle < 0) newAngle += 360;
    while (newAngle >= 360) newAngle -= 360;

    return newAngle;
}

function showHoverLabel(metricName, endpointID) {
    // Clear any existing timeout
    clearTimeout(hoverTimeout);

    // Remove existing label
    svg.selectAll('.hover-label').remove();

    // Add new label at bottom center
    // Create text element
    const label = svg.append('text')
        .attr('class', 'hover-label')
        .attr('x', centerX)
        .attr('y', height - 30)  // Move up a bit to make room for 2 lines
        .attr('text-anchor', 'middle')
        .style('font-size', '14px')
        .style('fill', '#fff')
        .style('opacity', 0);

    // Add first line (endpoint)
    label.append('tspan')
        .attr('x', centerX)
        .attr('dy', 0)
        .text(endpointID);

    // Add second line (metric)
    label.append('tspan')
        .attr('x', centerX)
        .attr('dy', '1.2em')  // Move down one line
        .text(metricName);

    // Fade in
    label.transition()
        .duration(200)
        .style('opacity', 1);
}

function hideHoverLabel() {
    // svg.selectAll('.hover-label').remove();
    hoverTimeout = setTimeout(() => {
        svg.selectAll('.hover-label')
            .transition()
            .duration(500)  // Slower fade out
            .style('opacity', 0)
            .remove();
    }, 1000);  // Wait before starting fade
}

function showPulseMeta(pulseData) {
    // Remove existing metadata display
    svg.selectAll('.pulse-metadata').remove();

    // Format the metadata
    const lines = [
        `Type: ${pulseData.type}`,
        `Metric: ${pulseData.metric}`,
        `Ring: ${pulseData.ring} (D${pulseData.dimension})`,
        `Intensity: ${(pulseData.intensity * 100).toFixed(0)}%`,
        `Duration: ${(pulseData.duration / 1e9).toFixed(1)}s`,
        `Endpoint: ${pulseData.endpoint}`
    ];

    // Create background box
    const boxHeight = lines.length * 16 + 10;
    svg.append('rect')
        .attr('class', 'pulse-metadata')
        .attr('x', 10)
        .attr('y', 10)
        .attr('width', 200)
        .attr('height', boxHeight)
        .style('fill', 'none')
        .style('stroke', 'none')
        .style('stroke-width', 1)
        .style('rx', 4)
        .style('opacity', 0)
        .transition()
        .duration(200)
        .style('opacity', 0.2);

    // Click-anywhere to dismiss
    svg.on('click.metadata', function(event) {
        // Only if clicking on background, not another pulse
        if (event.target.tagName === 'svg') {
            svg.selectAll('.pulse-metadata').remove();
            svg.on('click.metadata', null); // Remove this listener
        }
    });

    // Add text lines
    lines.forEach((line, i) => {
        svg.append('text')
            .attr('class', 'pulse-metadata')
            .attr('x', 20)
            .attr('y', 30 + (i * 16))
             //.attr('y', height - boxHeight + 20 + (i * 16))
            .style('fill', '#00E676FF')
            .style('font-size', '11px')
            .style('font-family', 'monospace')
            .style('pointer-events', 'none')
            .text(line)
            .style('opacity', 0)
            .transition()
            .duration(200)
            .style('opacity', 0.4);
    });

    // Auto-dismiss after 5 seconds with fade
    setTimeout(() => {
        svg.selectAll('.pulse-metadata')
            .transition()
            .duration(500)  // 500ms fade out
            .style('opacity', 0)
            .remove();  // Remove after fade completes
    }, 5000);
}

function eraseRadialGrid() {
    // Remove existing
    svg.selectAll('.radial-grid-line').remove();
    svg.selectAll('.radial-grid-circle').remove();
}

function drawRadialGrid() {
    // Reset for drawing
    eraseRadialGrid();

    // Clock spokes
    const numLines = 12; // 12 lines = every 30°
    const maxRadius = Math.min(width, height) / 2;

    for (let i = 0; i < numLines; i++) {
        const angle = (i * 360 / numLines - 90) * (Math.PI / 180);
        const bleedRadius = Math.max(width, height);
        const x2 = centerX + Math.cos(angle) * bleedRadius;
        const y2 = centerY + Math.sin(angle) * bleedRadius;

        svg.append('line')
            .attr('class', 'radial-grid-line')
            .attr('x1', centerX)
            .attr('y1', centerY)
            .attr('x2', x2)
            .attr('y2', y2)
            .style('stroke', '#00E676FF')
            .style('stroke-width', 1)
            .style('stroke-dasharray', '5,5')
            .style('opacity', 0.3);
    }

    // Optional: Concentric circles at intervals
    const numCircles = 4; // Grid granularity
    const circleSpacing = maxRadius / numCircles;

    for (let i = 0; i < numCircles; i++) {
        svg.append('circle')
            .attr('class', 'radial-grid-circle')
            .attr('cx', centerX)
            .attr('cy', centerY)
            .attr('r', i * circleSpacing)
            .style('fill', 'none')
            .style('stroke', '#00E676FF')
            .style('stroke-width', 1)
            .style('stroke-dasharray', '1,3')
            .style('opacity', 0.5);
    }
}

function animateD1ToD2Transition(d1Pulse, d2Pulse) {
    // Exit early if disabled
    if (!showRipple) return;

    const ring0Data = timeRings.find(r => r.ring === 0 && r.metric === d2Pulse.metric);
    const ring1Data = timeRings.find(r => r.ring === 1 && r.metric === d2Pulse.metric);

    if (!ring0Data || !ring1Data) return;

    const pos = getPulsePosition(1, d2Pulse.angle, d2Pulse.metric);

    // Create expanding ripple
    svg.append('circle')
        .attr('class', 'transition-ripple')
        .attr('cx', pos.x)
        .attr('cy', pos.y)
        .attr('r', 5)
        .style('fill', 'none')
        .style('stroke', '#e85ff8')
        .style('stroke-width', 2)
        .style('opacity', 0.2)
        .style('pointer-events', 'none')
        .transition()
        .duration(600)
        .attr('r', 20)  // Expand outward
        .style('opacity', 0)
        .remove();
}

function showHelpDialog() {
    // Remove existing help
    svg.selectAll('.help-dialog').remove();

    const helpText = [
        'Monteverdi Help',
        '',
        'Accents are created when a metric hits its max value.',
        'Pulses in the radar represent accent patterns in time.',
        'Inner rings in Dimension One (D1) rotate in 60 seconds.',
        'Outer rings in Dimension Two (D2) rotate in 10 minutes.',
        'Expect a slight delay on startup for patterns to warm.',
        '',
        'Each metric has its own ring in each Dimension.',
        'Pulses transition from D1 after 60 seconds,',
        'forming D2 patterns that rotate 10 minutes.',
        'Hover over pulses to see related groups',
        'and over rings for endpoint and metric names.',
        '',
        'Browse the way patterns interact by',
        'experimenting with the controls below!',
        'Hover over each element for help.',
        '',
        '[ ESC or ~click~ to close ]',
    ];

    const boxWidth = 400;
    const boxHeight = helpText.length * 20 + 20;

    // Background
    svg.append('rect')
        .attr('class', 'help-dialog')
        .attr('x', width/2 - boxWidth/2)
        .attr('y', 25)
        .attr('width', boxWidth)
        .attr('height', boxHeight)
        .style('fill', 'rgba(0, 0, 0, 0.4)')
        .style('stroke', '#00E676FF')
        .style('stroke-width', 2)
        .style('cursor', 'help')
        .style('rx', 8)
        .on('click', () => svg.selectAll('.help-dialog').remove());

    // Text
    helpText.forEach((line, i) => {
        svg.append('text')
            .attr('class', 'help-dialog')
            .attr('x', width/2)
            .attr('y', 45 + (i * 20))
            .attr('text-anchor', 'middle')
            .style('fill', i === 0 ? 'rgb(232,95,248)' : '#00E676FF')
            .style('font-size', i === 0 ? '18px' : '14px')
            .style('font-weight', i === 0 ? 'bold' : 'normal')
            .style('pointer-events', 'none')
            .text(line);
    });

    // Add ESC key listener
    const escHandler = function(event) {
        if (event.key === 'Escape' || event.key === 'Esc') {
            svg.selectAll('.help-dialog').remove();
            document.removeEventListener('keydown', escHandler);
        }
    };

    document.addEventListener('keydown', escHandler);
}

function updateTimeDisplay() {
    svg.selectAll('.time-display').remove();

    // const timeString = currentTime.toLocaleTimeString();
    const timeString = currentTime.toISOString()

    svg.append('text')
        .attr('class', 'time-display')
        .attr('x', width / 2)
        .attr('y', 20)
        .attr('text-anchor', 'middle')
        .style('fill', '#00E676FF')
        .style('opacity', 0.3)
        .style('font-size', '12px')
        .style('font-family', 'monospace')
        .style('pointer-events', 'none')
        .text(timeString);
}

// Initial time display, which is also updated each second.
updateTimeDisplay();

// Add center dot immediately
// (transparent in CSS by default)
svg.append("circle")
    .attr("cx", centerX)
    .attr("cy", centerY)
    .attr("r", 4)
    .attr("class", "center-dot")
    .style("fill", "#ccc");

// Help button in lower-left
const helpGroup = svg.append('g')
    .attr('class', 'help-button')
    .style('cursor', 'help')
    .on('click', showHelpDialog);

// Help Circle background
helpGroup.append('circle')
    .attr('cx', 20)
    .attr('cy', height - 20)
    .attr('r', 15)
    .style('fill', '#2d2d2d')
    .style('stroke', '#00E676FF')
    .style('opacity', 0.1)
    .style('stroke-width', 2);

// Help Question mark
helpGroup.append('text')
    .attr('x', 20)
    .attr('y', height - 20)
    .attr('text-anchor', 'middle')
    .attr('dominant-baseline', 'central')
    .style('fill', '#00E676FF')
    .style('opacity', 0.2)
    .style('font-size', '20px')
    .style('font-weight', 'bold')
    .style('pointer-events', 'none')
    .style('font-family', 'sans-serif')
    .text('?');

console.log('JavaScript loaded, waiting for WebSocket data...');