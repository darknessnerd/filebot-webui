<script setup>
import {ref, onMounted, watch} from 'vue';
import DirectoryBrowser from "./components/DirectoryBrowser.vue";
import FileBotStatus from "./components/FileBotStatus.vue";
import FileBotForm from "./components/FileBotForm.vue";
import { useAxios } from "./composable/useAxios.js";

// State for selected files and output directory
const selectedFiles = ref([]);
const outputDir = ref(null); // Set initial state to null for better handling
const inputDirectories = ref([]);
const outputDirectories = ref([]);

// Fetch directories from the endpoint on component mount
// Fetch directories using useAxios
const { data, loading, error, fetchData } = useAxios('directories'); // Adjust endpoint as necessary

// Watch for changes in the data
watch(data, (newData) => {
  if (newData) {
    inputDirectories.value = newData.inputDirectories || [];
    outputDirectories.value = newData.outputDirectories || [];
  }
});

// Handle file and directory selections
const handleSelectedFiles = (items) => {
  selectedFiles.value = items.length ? items : []; // Ensure an empty array if no items selected
};

const handleSelectedDirectories = (items) => {
  outputDir.value = items.length ? items[0] : null; // Ensure null if no directory is selected
};

const statusMessage = ref({ status: '', message: '' });

const handleFormSuccess = (message) => {
  statusMessage.value = message;
};

const handleFormError = (message) => {
  statusMessage.value = message;
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
        <!-- Input Directory Browser -->
        <div class="browser-content">
          <DirectoryBrowser
              :availableDirectories="inputDirectories"
              selection-mode="both"
              @update:selectedItems="handleSelectedFiles"
          />
        </div>

        <!-- Output Directory Browser -->
        <div class="browser-content">
          <DirectoryBrowser
              :availableDirectories="outputDirectories"
              selection-mode="directories"
              @update:selectedItems="handleSelectedDirectories"
          />
        </div>
      </div>

      <!-- Right Column for FileBotAction -->
      <div class="right-column">
        <FileBotForm
            :files="selectedFiles"
            :outputDirectory="outputDir"
            @formSuccess="handleFormSuccess"
            @formError="handleFormError"
        />
        <FileBotStatus :statusMessage="statusMessage" />
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
  @media (max-width: 768px) {
    overflow-y: auto;
    overflow-x: hidden;
  }
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
  @media (max-width: 768px) {
    padding: 2px;
  }
}

.summary {
  display: flex;
  gap: 16px;
  font-size: 14px;
  background-color: $summary-background-color;
  padding: 6px 12px;
  border-radius: $border-radius;
  @media (max-width: 768px) {
    padding: 2px;
    gap: 2px;
    font-size: 8px;
  }
}

.main-content {
  display: flex;
  flex-direction: row;
  flex-grow: 1;
  padding: 20px;
  gap: 20px;
  overflow-y: hidden;
  overflow-x: hidden;
  @media (max-width: 768px) {
    overflow-y: auto;
    overflow-x: hidden;
    flex-direction: column;
  }
}

.left-column {
  display: flex;
  flex-direction: column;
  flex-grow: 1;
  gap: 15px;
  flex-basis: 30%;
}

.right-column {
  display: flex;
  flex-direction: row;
  background-color: $card-background-color;
  border-radius: $border-radius;
  padding: 20px;
  box-shadow: 0 4px 8px $shadow-light;
  flex-basis: 70%;
  overflow-y: hidden;
  overflow-x: hidden;
  @media (max-width: 768px) {
    flex-direction: column; /* Switch to column layout for smaller screens like tablets and mobiles */
    flex-grow: 1;
    overflow-y: hidden;
    overflow-x: auto;
  }
}

.browser-content {
  overflow-y: hidden;
  padding-top: 0;
  background-color: $card-background-color;
  border-radius: $border-radius;
  box-shadow: 0 2px 4px $shadow-light;
  display: flex;
  flex-direction: column;
}

/* Responsive Design */
@media (max-width: 768px) {
  .main-content {
    flex-direction: column;
  }
  .summary {
    flex-direction: column;
  }
}
</style>
