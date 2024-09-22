import { ref } from 'vue';
import axios from 'axios';


export function useAxios(url, options = { }, fetchOptions = { immediate: true }) {
    const data = ref(null); // Data will be stored here
    const loading = ref(false); // Loading state
    const error = ref(null); // Error state
    const fetchData = async (optionsHook = {}) => {
        loading.value = true; // Start loading
        error.value = null; // Reset the error
        options = { ...options, ...optionsHook};
        try {
            // Make the request using Axios
            const response = await axios({
                url: `${import.meta.env.VITE_API_BASE_URL}${url}`,
                timeout: 600000, // 600 seconds = 10 minutes
                method: options.method || 'GET', // Default method is GET
                ...options, // Spread other axios options like headers, data, params, etc.
            });
            data.value = response.data; // Set the data from the response
        } catch (err) {
            error.value = err.response.data || 'Unknown error'; // Catch and set the error message
        } finally {
            loading.value = false; // Stop loading
        }
    };

    // Check for the immediate option
    if (fetchOptions.immediate) {
        fetchData(); // Fetch data immediately if immediate is true
    }

    // Return reactive properties so you can use them in the component
    return {
        data,
        loading,
        error,
        fetchData, // Expose the fetchData method if you want to manually re-fetch
    };
}
