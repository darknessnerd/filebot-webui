<template>
  <div class="filebot-action-container">
    <!-- Form for configuring FileBot parameters -->
    <form @submit.prevent="createFilesWithParameters" class="filebot-form" :class="{ 'invalid-form': !isFormValid }">

      <!-- Mandatory Fields -->
      <div class="form-group">
        <select v-model="commonOptions.db" required>
          <option value="" disabled>Select Database 🌐</option>
          <option value="TheMovieDB::TV">📺 TheMovieDB::TV</option>
          <option value="TheTVDB">📅 TheTVDB</option>
          <option value="AniDB">🎌 AniDB</option>
          <option value="TheMovieDB">🎥 TheMovieDB</option>
          <option value="OMDb">🌐 OMDb</option>
          <option value="AcoustID">🎵 AcoustID</option>
          <option value="ID3">🎧 ID3</option>
        </select>
        <span class="tooltip">Select the database for fetching metadata.</span>
      </div>

      <div class="form-group">
        <select v-model="commonOptions.action" required>
          <option value="" disabled>Select Action ✨</option>
          <option value="move">🚚 Move</option>
          <option value="copy">📂 Copy</option>
          <option value="symlink">🔗 Symlink</option>
          <option value="hardlink">🔗 Hardlink</option>
          <option value="test">🧪 Test</option>
        </select>
        <span class="tooltip">Choose the action to perform on the files.</span>
      </div>

      <div class="form-group">
        <select v-model="commonOptions.conflict_resolution" required>
          <option value="" disabled>Select Conflict Resolution ⚠️</option>
          <option value="skip">🚫 Skip</option>
          <option value="replace">🔄 Replace</option>
          <option value="auto">🔧 Auto</option>
          <option value="index">🔢 Index</option>
          <option value="fail">❌ Fail</option>
        </select>
        <span class="tooltip">Specify what to do if a conflict occurs.</span>
      </div>

      <div class="form-group">
        <select v-model="commonOptions.log_level" required>
          <option value="" disabled>Select Log Level 📜</option>
          <option value="all">🗂️ All</option>
          <option value="fine">🔍 Fine</option>
          <option value="info">ℹ️ Info</option>
          <option value="warning">⚠️ Warning</option>
        </select>
        <span class="tooltip">Choose the level of detail for the log output.</span>
      </div>

      <!-- Optional Fields -->
      <div class="form-group">
        <input
            type="text"
            id="format"
            v-model="commonOptions.format"
            placeholder="Format 🏷️"
            title="Specify the format for the files (e.g., {n} - {s00e00} - {t})" />
        <span class="tooltip">Specify the format for the files (e.g., {n} - {s00e00} - {t})</span>
      </div>

      <div class="form-group">
        <input
            type="text"
            id="filter"
            v-model="commonOptions.filter"
            placeholder="Filter 🔍"
            title="Apply a filter to the files (e.g., *.mkv)" />
        <span class="tooltip">Apply a filter to the files (e.g., *.mkv)</span>
      </div>

      <div class="form-group">
        <input
            type="text"
            id="query"
            v-model="optionalOptions.query"
            placeholder="Manual Lookup 🔍"
            title="Manual lookup query expression (e.g., movie name or ID)" />
        <span class="tooltip">Manual lookup query expression (e.g., movie name or ID)</span>
      </div>

      <div class="form-group checkbox-group">
        <div class="label" for="recursive">🌲 Recursive</div>
        <input
            type="checkbox"
            id="recursive"
            v-model="optionalOptions.recursive" />

        <span class="tooltip">Select this option to include files in subdirectories.</span>
      </div>

      <div class="form-actions">
        <button type="submit" :class="{ 'disabled-button': !isFormValid }">Submit 🚀</button>
      </div>
    </form>

    <!-- Fancy Error/Success Message Box Styled Like a Terminal -->
    <div v-if="statusMessage" class="terminal-box">
      <p class="terminal-header">🚀 FileBot Status</p>
      <pre class="terminal-output">{{ statusMessage }}</pre>
    </div>
  </div>
</template>


<script setup>
import { defineProps, ref, watch } from 'vue';
import axios from 'axios';

const props = defineProps({
  files: {
    type: Array,
    default: () => []
  },
  outputDirectory: {
    type: String,
    default: ''
  }
});

const commonOptions = ref({
  db: 'TheMovieDB', // Default database value
  format: '',
  action: 'test', // Default action value
  filter: '',
  conflict_resolution: 'skip', // Default conflict resolution value
  log_level: 'all' // Default log level value
});

const optionalOptions = ref({
  query: '',
  recursive: false,
  check: false
});

const statusMessage = ref('');


const validateForm = (options) => {
  return options.db && options.action && options.conflict_resolution && options.log_level;
};
const isFormValid = ref(true);
// Watch for changes in commonOptions to validate form
watch(commonOptions, (newValue) => {
  isFormValid.value = validateForm(newValue);
}, { deep: true });



// Function to create files and parameters
const createFilesWithParameters = async () => {
  if (!isFormValid.value) {
    return; // Prevent form submission if invalid
  }

  try {
    // Prepare data for API call
    const requestBody = {
      files: props.files,
      outputDirectory: props.outputDirectory,
      ...commonOptions.value,
      ...optionalOptions.value
    };

    // Send data to the server
    const response = await axios.post('execute-filebot', requestBody, {
      headers: {
        'Content-Type': 'application/json'
      }
    });

    statusMessage.value = '✅ ' + (response.data.message + response.data.output  || 'Operation successful!');
  } catch (error) {
    console.error('Error creating files and parameters:', error);
    statusMessage.value = '❌ ' + (error.response?.data?.errors?.join('\n') || 'Error executing FileBot.');
  }
};
</script>

<style scoped lang="scss">
@import "../variables";

.filebot-action-container {
  display: flex;
  flex-direction: row; /* By default, keep a row layout for larger screens */
  padding: 4px;
  overflow-y: auto;
  overflow-x: hidden;
  flex-grow: 1;

  @media (max-width: 768px) {
    flex-direction: column; /* Switch to column layout for smaller screens like tablets and mobiles */
  }
}

.filebot-form {
  display: flex;
  flex-direction: column;
  gap: 8px;
  flex-basis: 25%;
  position: relative;
  min-height: 220px;
  @media (max-width: 1024px) {
    flex-basis: 25%; /* Take full width on smaller screens */
  }
  @media (max-width: 768px) {
    flex-basis: 25%; /* Take full width on smaller screens */
  }
}

.form-group {
  display: flex;
  flex-direction: column;
  position: relative;
}

.checkbox-group {
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: 8px;

  @media (max-width: 768px) {
    flex-direction: column; /* Stack checkbox and label vertically on mobile */
    align-items: flex-start;
  }
}

input, select {
  padding: 4px;
  border: 1px solid $border-color;
  border-radius: $border-radius;
  width: 100%;
  box-sizing: border-box;
  font-size: 12px;
}

.form-actions {
  background-color: $card-background-color;
  border-top: 1px solid $border-color;
  padding: 10px 0;
  display: flex;
  justify-content: flex-end;
}

button {
  padding: 6px 10px;
  font-size: 12px;
  background-color: $primary-color-dark;
  color: $text-color-light;
  border: none;
  border-radius: $border-radius;
  cursor: pointer;
  transition: background-color 0.3s ease;
}

button.disabled-button {
  background-color: $disabled-button-color;
  color: $disabled-button-text-color;
  cursor: not-allowed;
}

button:hover:not(.disabled-button) {
  background-color: $button-hover-color;
}

/* Terminal box styling for status message */
.terminal-box {
  background-color: $terminal-background-color;
  color: $terminal-text-color;
  font-family: 'Courier New', Courier, monospace;
  border-radius: $border-radius;
  padding: 8px;
  margin-left: 8px;
  flex-basis: 75%;

  @media (max-width: 1024px) {
    flex-basis: 100%; /* Terminal box takes full width on tablets */
    margin-left: 0;
  }
  @media (max-width: 768px) {
    flex-basis: 25%; /* Take full width on smaller screens */
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

.invalid-form {
  border: 1px solid $invalid-form-border-color;
}

/* Tooltip styling */
.tooltip {
  display: none;
  position: absolute;
  left: 100%;
  top: 50%;
  transform: translateX(8px) translateY(-50%);
  background-color: $tooltip-background-color;
  color: $tooltip-text-color;
  padding: 4px;
  border-radius: $border-radius;
  font-size: 10px;
  white-space: nowrap;
  z-index: 10;

  @media (max-width: 768px) {
    left: auto;
    right: 0;
    transform: translateX(0) translateY(0); /* Adjust tooltip placement for mobile */
  }
}

.form-group:hover .tooltip {
  display: block;
}
</style>
