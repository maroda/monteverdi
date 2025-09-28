// index.js - Simple D3.js example with circles

// Set up dimensions
const width = 800;
const height = 800;

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

function updatePulsesFromBackend(backendData) {
    // Filter data
    const filteredData = backendData.filter(d => {
        const dimension = d.dimension || 1;
        const ring = d.ring || 0;
        return (dimension === 1 && ring === 0) || (dimension === 2 && ring >= 1);
    });

    // console.log('Filtered data count:', filteredData.length);

    //if (filteredData.length > 0) {
        // console.log('First item structure:', filteredData[0]);
    //}

    // Simple data join with good keys
    const pulses = svg.selectAll('.pulse')
        //.data(filteredData, d => `${d.metric}-${d.startTime || Date.now()}`);
        .data(filteredData, d => `${d.metric}-${d.startTime}-${d.dimension}`)

    // Remove dots that are no longer in data
    pulses.exit().remove();

    // Add new dots
    pulses.enter()
        .append('circle')
        .attr('class', d => `pulse pulse-${d.type}`)
        .attr('cx', d => getPulsePosition(d.ring, d.angle, d.metric).x)
        .attr('cy', d => getPulsePosition(d.ring, d.angle, d.metric).y)
        .attr('r', d => (d.intensity || 0.5) * 2 + 3);

    // Update positions for existing dots
    pulses
        .attr('cx', d => getPulsePosition(d.ring, d.angle, d.metric).x)
        .attr('cy', d => getPulsePosition(d.ring, d.angle, d.metric).y);
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
    const ringSpacing = 15;

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
        // console.log(`No ring found for ring:${ring}, metric:${metric}`);
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

    // console.log('Drawing', timeRings.length, 'rings');

    // Draw new rings
    svg.selectAll('.time-ring')
        .data(timeRings)
        .enter()
        .append('circle')
        .attr('class', 'time-ring')
        .attr('cx', centerX)
        .attr('cy', centerY)
        .attr('r', d => d.radius)
        .style('fill', 'none')
        .style('stroke', '#ddd')
        .style('stroke-width', 1);
}

// Add center dot immediately
svg.append("circle")
    .attr("cx", centerX)
    .attr("cy", centerY)
    .attr("r", 4)
    .attr("class", "center-dot")
    .style("fill", "#333");

console.log('JavaScript loaded, waiting for WebSocket data...');