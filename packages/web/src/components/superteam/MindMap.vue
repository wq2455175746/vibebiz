<script setup lang="ts">
const branches = [
  { title: '场景', leaves: ['支付失败反馈', '登录异常', '导出超时'] },
  { title: '用户', leaves: ['运营人员', '财务审核', '外部客户'] },
  { title: '价值', leaves: ['降低人工排查', '缩短响应时间', '提升续费率'] },
  { title: '风险', leaves: ['数据隐私', '噪声误判', '模型成本'] },
]
</script>

<template>
  <div class="super-mindmap">
    <div class="super-mindmap-root">需求洞察</div>
    <div class="super-mindmap-branches">
      <section v-for="branch in branches" :key="branch.title" class="super-mindmap-branch">
        <h4>{{ branch.title }}</h4>
        <ul>
          <li v-for="leaf in branch.leaves" :key="leaf">{{ leaf }}</li>
        </ul>
      </section>
    </div>
  </div>
</template>
