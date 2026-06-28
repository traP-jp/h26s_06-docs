export const nodeVertexShader = `
varying vec3 vColor;
varying vec3 vObjectPosition;
varying vec3 vViewNormal;

void main() {
    vColor = instanceColor;
    vObjectPosition = position;
    vec4 modelViewPosition = modelViewMatrix * instanceMatrix * vec4(position, 1.0);
    vViewNormal = normalize(normalMatrix * mat3(instanceMatrix) * normal);
    gl_Position = projectionMatrix * modelViewPosition;
}
`;

export const nodeFragmentShader = `
precision highp float;

uniform float uTime;

varying vec3 vColor;
varying vec3 vObjectPosition;
varying vec3 vViewNormal;

float hash(vec3 p) {
    return fract(sin(dot(p, vec3(127.1, 311.7, 74.7))) * 43758.5453123);
}

void main() {
    vec3 normal = normalize(vViewNormal);
    float facing = abs(normal.z);

    float outerGlow = pow(facing, 0.24) * 0.16;
    float glow = pow(facing, 0.82);
    float softHalo = pow(facing, 3.8) * 0.28;
    float nucleus = smoothstep(0.988, 1.0, facing);

    vec3 cell = floor(normalize(vObjectPosition) * 22.0);
    float seed = hash(cell);
    float shimmerWave = sin(uTime * (1.5 + seed * 2.8) + seed * 18.0);
    float shimmer = smoothstep(0.78, 1.0, shimmerWave) * smoothstep(0.64, 1.0, seed) * pow(facing, 2.2);

    float alpha = outerGlow + glow * 0.26 + softHalo + nucleus * 0.82 + shimmer * 0.14;
    alpha *= smoothstep(0.08, 0.34, length(vObjectPosition));

    if (alpha < 0.006) discard;

    vec3 nucleusColor = vec3(0.96, 0.98, 1.0);
    vec3 color = vColor * (0.3 + outerGlow * 0.68 + glow * 0.98 + softHalo * 0.48) + nucleusColor * (nucleus * 1.55 + shimmer * 0.58);
    gl_FragColor = vec4(color, alpha);
}
`;
