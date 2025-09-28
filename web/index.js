// index.js - Simple D3.js example with circles

// Set up dimensions
const width = 800;
const height = 800;
const margin = { top: 20, right: 20, bottom: 20, left: 20 };

// Create the SVG element
const svg = d3.select("#chart")
    .attr("width", width)
    .attr("height", height);

// Radar setup - concentric circles for different time scales
const centerX = width / 2;
const centerY = height / 2;

// Define our concentric circles (time scales)
const timeRings = [
    { radius: 120, label: "Last 60 seconds", strokeWidth: 2, color: "#ddd" },
    { radius: 180, label: "Last 10 minutes", strokeWidth: 1.5, color: "#bbb" },
    { radius: 240, label: "Last hour", strokeWidth: 1, color: "#999" }
];

// Pulse data
let pulseData = [];

// WebSocket connection
const ws = new WebSocket('ws://localhost:8090/ws')

ws.onmessage = function(event) {
    const data = JSON.parse(event.data);

    // Log raw data before any processing
    const amphibrachs = data.filter(d => d.type === 'amphibrach');
    if (amphibrachs.length > 0) {
        console.log('Raw amphibrach data from WebSocket:', amphibrachs.map(a => ({
            ring: a.ring,
            Ring: a.Ring,  // Check both cases
            angle: a.angle,
            type: a.type
        })));
    }

    updatePulsesFromBackend(data);
};

function updatePulsesFromBackend(backendData) {
    // Filter data
    const filteredData = backendData.filter(d => {
        const dimension = d.Dimension || d.dimension || 1;
        const ring = d.Ring || d.ring || 0;
        return (dimension === 1 && ring === 0) || (dimension === 2 && ring >= 0);
    });

    // Simple data join with good keys
    const pulses = svg.selectAll('.pulse')
        // .data(filteredData, d => `${d.metric}-${d.type}-${d.ring}`);
        .data(filteredData, d => `${d.metric}-${d.startTime}`)

    // Remove dots that are no longer in data
    pulses.exit().remove();

    console.log('Filtered data count:', filteredData.length);
    console.log('Sample filtered data:', filteredData.slice(0, 3));

    // Add new dots
    pulses.enter()
        .append('circle')
        .attr('class', d => `pulse pulse-${d.type}`)
        .attr('cx', d => getPulsePosition(d.ring, d.angle).x)
        .attr('cy', d => getPulsePosition(d.ring, d.angle).y)
        .attr('r', d => d.intensity * 2 + 3);

    // Update positions for existing dots
    pulses
        .attr('cx', d => getPulsePosition(d.ring, d.angle).x)
        .attr('cy', d => getPulsePosition(d.ring, d.angle).y);
}

// Draw the concentric circles (time rings)
svg.selectAll(".time-ring")
    .data(timeRings)
    .enter()
    .append("circle")
    .attr("class", "time-ring")
    .attr("cx", centerX)
    .attr("cy", centerY)
    .attr("r", d => d.radius)
    .style("fill", "none")
    .style("stroke", d => d.color)
    .style("stroke-width", d => d.strokeWidth);

timeRings.forEach((ring, ringIndex) => {
    const markers = [0, 90, 180, 270]; // 12, 3, 6, 9 o'clock
    markers.forEach(markerAngle => {
        const pos = getPulsePosition(ringIndex, markerAngle);
        svg.append("circle")
            .attr("cx", pos.x)
            .attr("cy", pos.y)
            .attr("r", 2)
            .attr("class", "time-marker")
            .style("fill", "#666");
    });
});

// Add center dot
svg.append("circle")
    .attr("cx", centerX)
    .attr("cy", centerY)
    .attr("r", 4)
    .attr("class", "center-dot");

function getPulsePosition(ring, angle) {
    const radius = timeRings[ring].radius;
    const radians = (angle - 90) * (Math.PI / 180); // Adjust for D3's coordinate system
    return {
        x: centerX + Math.cos(radians) * radius,
        y: centerY + Math.sin(radians) * radius
    };
}

// Draw pulses
const pulses = svg.selectAll(".pulse")
    .data(pulseData)
    .enter()
    .append("circle")
    .attr("class", d => `pulse pulse-${d.type}`)
    .attr("cx", d => getPulsePosition(d.ring, d.angle).x)
    .attr("cy", d => getPulsePosition(d.ring, d.angle).y)
    .attr("r", d => d.intensity * 8 + 3); // Size based on intensity

// Add ring labels
svg.selectAll(".ring-label")
    .data(timeRings)
    .enter()
    .append("text")
    .attr("class", "ring-label")
    .attr("x", centerX + 10)
    .attr("y", d => centerY - d.radius - 10)
    .text(d => d.label);