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

// Display rings
let timeRings = [];
let knownMetrics = new Set();
let initialized = false;
let pulseLengthMultiplier = 0.4;
let ringSpacing = 12;

// WebSocket connection
const ws = new WebSocket('ws://localhost:8090/ws')

ws.onmessage = function(event) {
    const data = JSON.parse(event.data);
    console.log('Received data, length:', data.length);

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
        console.log('Amphibrach data:', amphibrachs[0]);
        console.log('Ring:', amphibrachs[0].ring, 'Angle:', amphibrachs[0].angle);
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
            return `rotate(${d.angle} ${pos.x} ${pos.y})`;
        })
        .style('opacity', d => 0.2 + (d.intensity || 0.5) * 0.4);

    // Update positions for existing dots
    pulses
        .attr('cx', d => getPulsePosition(d.ring, d.angle, d.metric).x)
        .attr('cy', d => getPulsePosition(d.ring, d.angle, d.metric).y)
        .attr('transform', d => {
            const pos = getPulsePosition(d.ring, d.angle, d.metric);
            return `rotate(${d.angle} ${pos.x} ${pos.y})`;
        });
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
    const baseRadius = 60;
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

    const radius = ringData.radius;
    const radians = (angle - 90) * (Math.PI / 180);
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
            showHoverLabel(d.metric);
            d3.select(this).style('stroke', '#888').style('stroke-width', 2);
        })
        .on('mouseout', function(event, d) {
            hideHoverLabel();
            d3.select(this).style('stroke', '#555').style('stroke-width', 1);
        });
}

let hoverTimeout;

function showHoverLabel(metricName) {
    // Clear any existing timeout
    clearTimeout(hoverTimeout);

    // Remove existing label
    svg.selectAll('.hover-label').remove();

    // Add new label at bottom center
    svg.append('text')
        .attr('class', 'hover-label')
        .attr('x', centerX)
        .attr('y', height - 20)
        .attr('text-anchor', 'middle')
        .style('font-size', '14px')
        .style('fill', '#fff')
        .style('opacity', 0)  // Start invisible
        .text(metricName)
        .transition()
        .duration(200)
        .style('opacity', 1);  // Fade in
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