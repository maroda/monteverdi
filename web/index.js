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
let timeRings = [];
let knownMetrics = new Set();
let initialized = false;
let pulseLengthMultiplier = 0.4;
let ringSpacing = 12;
let lastReceivedData = [];
let r0expScale = false;
let r1expScale = false;
let highlightThreshold = 3;
let currentHighlightedPulse = null;

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

function updatePulsesFromBackend(backendData) {
    // Debug amphibrach data
    const amphibrachs = backendData.filter(d => d.type === 'amphibrach');
    if (amphibrachs.length > 0) {
        // console.log('Amphibrach data:', amphibrachs[0]);
        // console.log('Ring:', amphibrachs[0].ring, 'Angle:', amphibrachs[0].angle);
    }

    // Filter data
    const filteredData = backendData.filter(d => {
        const dimension = d.dimension || 1;
        const ring = d.ring || 0;
        return (dimension === 1 && ring === 0) || (dimension === 2 && ring === 1);
    });

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

let hoverTimeout;

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

// Add center dot immediately
svg.append("circle")
    .attr("cx", centerX)
    .attr("cy", centerY)
    .attr("r", 4)
    .attr("class", "center-dot")
    .style("fill", "#ccc");

console.log('JavaScript loaded, waiting for WebSocket data...');