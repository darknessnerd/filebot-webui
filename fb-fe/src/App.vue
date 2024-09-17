<script setup>
import DirectoryBrowser from "./components/DirectoryBrowser.vue";
import FileBotAction from "./components/FileBotAction.vue";
import { ref } from 'vue';

// State for selected files and output directory
const selectedFiles = ref([]);
const outputDir = ref(null); // Set initial state to null for better handling

// Handle file and directory selections
const handleSelectedFiles = (items) => {
  selectedFiles.value = items.length ? items : []; // Ensure an empty array if no items selected
};

const handleSelectedDirectories = (items) => {
  outputDir.value = items.length ? items[0] : null; // Ensure null if no directory is selected
};
</script>

<template>
  <div class="app-container">
    <!-- Enhanced App Header with better UX -->
    <header class="app-header">
      <h1>🗃️ FileBot</h1>
      <div class="summary">
        <span>Files: 🗃️ {{ selectedFiles.length > 0 ? selectedFiles.length : 'None' }}</span>
        <span>
          Directory: 📁
          {{ outputDir ? outputDir.path + '\\' + outputDir.name : 'None' }}
        </span>
      </div>
    </header>

    <div class="main-content">
      <!-- Two-column layout -->
      <div class="left-column">
        <!-- File Browser -->
        <div class="browser-content">
          <DirectoryBrowser selection-mode="files" @update:selectedItems="handleSelectedFiles" />
        </div>

        <!-- Directory Browser -->
        <div class="browser-content">
          <DirectoryBrowser selection-mode="directories" @update:selectedItems="handleSelectedDirectories" />
        </div>
      </div>

      <!-- Right Column for FileBotAction -->
      <div class="right-column">
        <FileBotAction :files="selectedFiles" :output-directory="outputDir" />
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
@import "./variables.scss";
.app-container {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background-color: $background-color-dark;
}

.app-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 15px;
  background-color: $header-background-color;
  color: $header-text-color; // Adjusted text color for contrast
  font-size: 18px;
  font-weight: $font-weight-bold;
  box-shadow: 0 2px 4px $shadow-light;
}

.summary {
  display: flex;
  gap: 16px;
  font-size: 14px;
  background-color: $summary-background-color;
  padding: 6px 12px;
  border-radius: $border-radius;
}

.main-content {
  display: flex;
  flex-grow: 1;
  padding: 20px;
  gap: 20px;
  overflow-y: hidden;
  overflow-x: hidden;
}

.left-column {
  display: flex;
  flex-direction: column;
  flex: 1;
  gap: 15px;
  flex-basis: 30%;
}

.right-column {
  display: flex;
  flex-direction: column;
  background-color: $card-background-color;
  border-radius: $border-radius;
  padding: 20px;
  box-shadow: 0 4px 8px $shadow-light;
  overflow-y: hidden;
  flex-basis: 70%;
}

.browser-content {
  overflow-y: auto;
  max-height: 300px;
  padding-top: 0;
  flex-grow: 1;
  background-color: $card-background-color;
  border-radius: $border-radius;
  box-shadow: 0 2px 4px $shadow-light;
}

/* Responsive Design */
@media (max-width: 768px) {
  .main-content {
    flex-direction: column;
  }

  .left-column, .right-column {
    width: 100%;
  }

  .summary {
    flex-direction: column;
  }
}

</style>

