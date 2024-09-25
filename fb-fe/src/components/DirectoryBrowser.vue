<template>
  <div class="directory-browser">
    <!-- Enhanced Header -->
    <div class="header">
      <div class="header-title">
        <div class="ellipsized-text">📁 {{ currentPath }}</div>
      </div>
      <div class="header-buttons">
        <!-- Combo box for selecting the root directory -->
        <select v-model="currentPath" @change="browseDirectory('')" class="header-button" aria-label="Select Directory">
          <option v-for="dir in availableDirectories" :key="dir" :value="dir">
            {{ dir }}
          </option>
        </select>

        <button @click="goUp" :disabled="currentPath === rootPath" class="header-button" aria-label="Go Up">
          <span class="icon">⬆️</span>
          <span class="tooltip">Go Up</span>
        </button>
        <button @click="refreshDirectory" class="header-button" aria-label="Refresh Directory">
          <span class="icon">🔄</span>
          <span class="tooltip">Refresh</span>
        </button>
        <button @click="toggleCollapse" class="header-button" aria-label="Toggle Collapse">
          <span class="icon">{{ isCollapsed ? '▶' : '▼' }}</span>
          <span class="tooltip">{{ isCollapsed ? 'Expand' : 'Collapse' }}</span>
        </button>

        <div v-if="props.selectionMode === 'both' || props.selectionMode === 'directories'" class="header-button current-directory-select">
          <label class="checkbox-label">
            <span class="icon">📦</span> <!-- Icon for the checkbox -->
            <input type="checkbox" :checked="isSelectedDirectory" @change="toggleCurrentDirectorySelection" />
            <span class="tooltip">Select Current Directory</span>
          </label>
        </div>
      </div>
    </div>

    <!-- Useful Info in Header -->
    <div class="directory-info" v-if="files && files.length > 0">
      <span>{{ files.length }} items ({{ files.filter(item => item.type === 'directory').length }} directories, {{ files.filter(item => item.type === 'file').length }} files)</span>
    </div>

    <!-- Collapsible Content -->
    <div v-show="!isCollapsed" class="content">
      <div v-if="loading" class="status">⏳ Loading...</div>
      <div v-if="error" class="status error">❌ Error: {{ error }}</div>

      <ul v-if="!loading && files">
        <li v-for="item in files" :key="item.name">
          <div class="item-row"
               @click.prevent.stop="item.type === 'directory' ? browseDirectory(item.name) : null"
               :style="{ cursor: item.type === 'directory' ? 'pointer' : 'default' }">
            <strong class="item-icon">{{ item.type === 'directory' ? '📂' : '📄' }}</strong>
            <!-- Added ellipsized-text class -->
            <div class="item-name ellipsized-text">{{ item.name }}</div>
            <div class="item-checkbox-wrapper">
              <input type="checkbox"
                     :disabled="!isCheckboxAvailable(item)"
                     :checked="isSelected(item)"
                     :value="item.name"
                     class="item-checkbox"
                     @click.stop="toggleSelection(item)" />
            </div>
            <div class="item-tooltip">{{ item.path }}</div>
          </div>
        </li>
      </ul>
    </div>
  </div>
</template>

<script setup>
import {ref, watch, computed, onMounted} from 'vue';
import {useAxios} from "../composable/useAxios.js";

// Define props including availableDirectories for combo box
const props = defineProps({
  availableDirectories: {
    type: Array,
    default: () => ['.']
  },
  selectionMode: {
    type: String,
    default: 'both',
    validator: value => ['both', 'files', 'directories'].includes(value)
  }
});

const rootPath = ref(props.availableDirectories[0]); // Starting directory from the prop
const currentPath = ref(rootPath.value); // Track current directory
const currentDirectory = ref({}); // Track current directory
const selectedItems = ref([]); // Store selected directories/files
const emit = defineEmits(['update:selectedItems']); // Define the emit event
const isCollapsed = ref(false); // Track if the component is collapsed

// Data from API
const files = ref([]);
onMounted(() => {
  currentPath.value = props.availableDirectories[0]; // Set the first element as the default selected
});
// Check if checkbox should be available
const isCheckboxAvailable = (item) => {
  return props.selectionMode === 'both' ||
      (props.selectionMode === 'files' && item.type === 'file') ||
      (props.selectionMode === 'directories' && item.type === 'directory');
};

// Check if the item is selected
const isSelected = (item) => {
  return selectedItems.value.some(selectedItem =>
      selectedItem.name === item.name && selectedItem.type === item.type
  );
};

// Fetch the directory data and update the state
const {data, loading, error, fetchData} = useAxios('explore', {
  params: {path: currentPath.value},
});

// Watch for changes in the data and update files & currentPath
watch(data, (newData) => {
  if (newData) {
    currentPath.value = newData.currentPath; // Update current path
    currentDirectory.value = {name: newData.name, path: newData.path}
    files.value = newData.files; // Update file list
  }
});

// Watch for changes in selectedItems and handle them (if needed)
watch(selectedItems, (newSelected) => {
  emit('update:selectedItems', newSelected); // Emit the selectedItems
}, {deep: true});

watch(currentPath, () => {
  selectedItems.value = []; // Clear selection on path change
});

// Navigate into a directory
const browseDirectory = (folder) => {
  const newPath = folder ? `${currentPath.value}/${folder}` : currentPath.value;
  fetchData({
    params: {path: newPath},
  });
};

// Go up one directory level
const goUp = () => {
  const parentPath = currentPath.value.split('/').slice(0, -1).join('/');
  const newPath = parentPath || rootPath.value; // Prevent going above the root path
  fetchData({
    params: {path: newPath},
  });
};

// Toggle item selection
const toggleSelection = (item) => {
  const index = selectedItems.value.findIndex(selectedItem => selectedItem.name === item.name && selectedItem.type === item.type);
  if (index === -1) {
    selectedItems.value.push(item); // Add if not selected
  } else {
    selectedItems.value.splice(index, 1); // Remove if already selected
  }
};

// Toggle collapse state
const toggleCollapse = () => {
  isCollapsed.value = !isCollapsed.value;
};

// Refresh the current directory
const refreshDirectory = () => {
  fetchData({
    params: {path: currentPath.value},
  });
};

// Computed property to check if the current directory is selected
const isSelectedDirectory = computed(() => {
  return selectedItems.value.some(item => item.type === 'directory' && item.name === currentPath.value.split('/').pop());
});

// Toggle the selection of the current directory
const toggleCurrentDirectorySelection = (event) => {
  const isChecked = event.target.checked;
  const currentDirName = currentDirectory.value.name;
  const currentDirItem = {name: currentDirName, path: currentDirectory.value.path, type: 'directory'};
  if (isChecked) {
    selectedItems.value = [currentDirItem]; // Select only the current directory
  } else {
    selectedItems.value = selectedItems.value.filter(item => !(item.type === 'directory' && item.name === currentDirName));
  }
};
</script>

<style scoped lang="scss">
@import "../variables";

.directory-browser {
  flex-grow: 0;
  flex-shrink: 1;
  display: flex;
  flex-direction: column;
  overflow-y: hidden;
  overflow-x: hidden;
  background-color: $card-background-color;
  border-radius: $border-radius;
  box-shadow: 0 2px 4px $shadow-light;
}

.header {
  flex-basis: 10%;
  display: flex;
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
  padding: 4px 8px;
  background-color: var(--header-background-color);
  border-bottom: 1px solid var(--border-color);

}

.header-title {
  display: flex;
  flex-direction: row;
  align-items: center;
  font-weight: var(--font-weight-bold);
  font-size: 12px;
  color: var(--header-text-color);
  min-width: 0;
  flex-grow: 1;
  flex-shrink: 1;
}

.ellipsized-text {

  display: block;/* Change it as per your requirement. */
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 180px;
}

.header-buttons {
  display: flex;
  flex-direction: row;
  gap: 4px;
  flex-grow: 0;
  flex-shrink: 1;
  justify-content: flex-end;
}

.header-button {
  position: relative;
  padding: 4px;
  font-size: 12px;
  cursor: pointer;
  background-color: var(--background-color-dark);
  border-radius: 4px;
  border: 1px solid var(--border-color);
  color: var(--text-color-light);
}

.header-button .icon {
  font-size: 12px;
}

.header-button .tooltip {
  position: absolute;
  top: 100%;
  left: 50%;
  transform: translateX(-50%);
  background-color: var(--tooltip-background-color);
  color: var(--tooltip-text-color);
  padding: 4px;
  font-size: 8px;
  border-radius: 4px;
  white-space: nowrap;
  opacity: 0;
  visibility: hidden;
  transition: opacity 0.2s ease;
}

.header-button:hover .tooltip {
  opacity: 1;
  visibility: visible;
}

.checkbox-label {
  display: flex;
  align-items: center;
}

.checkbox-label input {
  margin-right: 6px;
}

.current-directory-select {
  display: flex;
  align-items: center;
}

.directory-info {
  font-size: 10px;
  padding: 4px 8px;
  background-color: var(--background-color-dark);
  color: var(--text-color-light);
}

.content {
  display: flex;
  flex-direction: row;
  padding: 5px;
  overflow-y: auto;
  overflow-x: hidden;
}

ul {
  list-style-type: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  flex-grow: 1;
}

li {
  margin: 2px 0;
}

.item-row {
  display: flex;
  align-items: center;
  padding: 3px 5px;
  border-radius: var(--border-radius);
  transition: background-color 0.2s ease;
  font-size: 12px;
  color: var(--text-color-dark);
}

.item-row:hover {
  background-color: var(--summary-background-color);
}

.item-icon {
  flex-shrink: 0;
  margin-right: 6px;
  font-size: 16px;
}

.item-name {
  flex-grow: 1;
  margin-right: 8px;
}

.item-checkbox-wrapper {
  display: flex;
}

.item-tooltip {
  position: absolute;
  left: 5%;
  bottom: 60%;
  background-color: var(--tooltip-background-color);
  color: var(--tooltip-text-color);
  padding: 4px;
  font-size: 10px;
  border-radius: 4px;
  white-space: nowrap;
  display: none;
}

.item-row:hover .item-tooltip {
  display: block;
}
</style>
