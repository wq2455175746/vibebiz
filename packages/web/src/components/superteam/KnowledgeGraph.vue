<script setup lang="ts">
const nodes = [
  { id: 'events', label: '事件源', x: 90, y: 80, kind: 'source' },
  { id: 'pay', label: '支付失败', x: 250, y: 50, kind: 'entity' },
  { id: 'export', label: '导出超时', x: 250, y: 160, kind: 'entity' },
  { id: 'retention', label: '续费率', x: 440, y: 50, kind: 'metric' },
  { id: 'ops', label: '运维效率', x: 440, y: 160, kind: 'metric' },
  { id: 'opportunity', label: '机会', x: 570, y: 105, kind: 'source' },
]

const edges = [
  { id: 'e1', x1: 124, y1: 80, x2: 216, y2: 50 },
  { id: 'e2', x1: 124, y1: 80, x2: 216, y2: 160 },
  { id: 'e3', x1: 284, y1: 50, x2: 406, y2: 50 },
  { id: 'e4', x1: 284, y1: 160, x2: 406, y2: 160 },
  { id: 'e5', x1: 474, y1: 50, x2: 536, y2: 105 },
  { id: 'e6', x1: 474, y1: 160, x2: 536, y2: 105 },
]
</script>

<template>
  <div class="super-graph">
    <svg viewBox="0 0 660 220" role="img" aria-label="需求知识图谱">
      <line
        v-for="edge in edges"
        :key="edge.id"
        :x1="edge.x1"
        :y1="edge.y1"
        :x2="edge.x2"
        :y2="edge.y2"
        class="super-graph-edge"
      />
      <g v-for="node in nodes" :key="node.id" :transform="`translate(${node.x}, ${node.y})`">
        <circle r="34" :class="['super-graph-node', `is-${node.kind}`]" />
        <text text-anchor="middle" dy="4" class="super-graph-label">{{ node.label }}</text>
      </g>
    </svg>
  </div>
</template>
