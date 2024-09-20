<template>
  <div v-if="statusMessage" class="terminal-box">
    <p class="terminal-header">🚀 FileBot Status</p>
    <pre class="terminal-output">{{ statusMessage }}</pre>
  </div>
</template>

<script setup>
import { defineProps } from 'vue';

const props = defineProps({
  statusMessage: {
    type: Object,
    default: () => ({ status: '', message: '' })
  }
});
</script>


<style scoped lang="scss">
@import "../variables";
/* Terminal box styling for status message */
.terminal-box {
  background-color: $terminal-background-color;
  color: $terminal-text-color;
  font-family: 'Courier New', Courier, monospace;
  border-radius: $border-radius;
  padding: 8px;
  margin-left: 8px;
  flex-basis: 75%;
  overflow-y: auto;
  overflow-x: hidden;
  @media (max-width: 1024px) {
    flex-basis: 100%; /* Terminal box takes full width on tablets */
    margin-left: 0;
  }
  @media (max-width: 768px) {
    flex-grow: 1;
    overflow-y: auto;
    overflow-x: hidden;
  }
}

.terminal-header {
  font-weight: $font-weight-bold;
  color: $terminal-header-color;
  font-size: 12px;
  margin-bottom: 5px;
}

.terminal-output {
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 12px;
}


</style>
