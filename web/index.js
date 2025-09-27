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
    const amphibrachs = data.filter(d => d.dimension === 2);
    console.log(`Amphibrachs received: ${amphibrachs.length}`, amphibrachs);

    // Check for stuck angles
    amphibrachs.forEach(a => {
        if (a.angle > 300 && a.angle < 360) { // Around 1/6th rotation area
            console.log("Stuck amphibrach:", a);
        }
    });

    updatePulsesFromBackend(data);
};

/*
function updatePulsesFromBackend(backendData) {
    // Remove old pulses
    svg.selectAll('.pulse').remove();

    // Add new pulses from backend
    svg.selectAll('.pulse')
        .data(backendData)
        .enter()
        .append('circle')
        .attr('class', d => `pulse pulse-${d.type}`)
        .attr('cx', d => getPulsePosition(d.ring, d.angle).x)
        .attr('cy', d => getPulsePosition(d.ring, d.angle).y)
        // change to make smaller: 2
        .attr('r', d => d.intensity * 2 + 3);
}

 */
function updatePulsesFromBackend(backendData) {
    // Remove old pulses
    svg.selectAll('.pulse').remove();

    // Filter based on dimension and ring
    const filteredData = backendData.filter(d => {
        const dimension = d.Dimension || d.dimension || 1;
        const ring = d.Ring || d.ring || 0;

        // D1 (iamb/trochee) only on inner ring (ring 0)
        if (dimension === 1) {
            return ring === 0;
        }
        // D2 (amphibrach) only on middle and outer rings (ring 1, 2)
        if (dimension === 2) {
            // return ring === 1 || ring === 2;
            return ring >= 0;
        }
        return false; // Filter out unknown dimensions
    });

    // Add new pulses from filtered data
    svg.selectAll('.pulse')
        .data(filteredData)
        .enter()
        .append('circle')
        .attr('class', d => `pulse pulse-${d.type}`)
        .attr('cx', d => getPulsePosition(d.ring, d.angle).x)
        .attr('cy', d => getPulsePosition(d.ring, d.angle).y)
        .attr('r', d => d.intensity * 2 + 3);
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

// Function to calculate pulse position
/*
function getPulsePosition(ring, angle) {
    const radius = timeRings[ring].radius;
    const radians = angle * (Math.PI / 180);
    return {
        x: centerX + Math.cos(radians) * radius,
        y: centerY + Math.sin(radians) * radius
    };
}

 */
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

// Animation function to move pulses around the rings
function animatePulses() {
    pulseData.forEach((pulse, i) => {
        // Update angle based on speed
        pulse.angle = (pulse.angle + pulse.speed) % 360;

        // Update position
        const pos = getPulsePosition(pulse.ring, pulse.angle);

        // Animate the pulse to new position
        d3.select(`.pulse:nth-child(${i + timeRings.length + 2})`) // Account for rings and center
            .transition()
            .duration(50)
            .ease(d3.easeLinear)
            .attr("cx", pos.x)
            .attr("cy", pos.y);
    });
}

// Start animation loop
setInterval(animatePulses, 50); // Update every 50ms for smooth motion

// Add ring labels
svg.selectAll(".ring-label")
    .data(timeRings)
    .enter()
    .append("text")
    .attr("class", "ring-label")
    .attr("x", centerX + 10)
    .attr("y", d => centerY - d.radius - 10)
    .text(d => d.label);