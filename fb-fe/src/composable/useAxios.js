import { ref } from 'vue';
import axios from 'axios';

const baseApiUrl = import.meta.env.VITE_API_BASE_URL;

export function useAxios(url, options = {}) {
    const data = ref(null); // Data will be stored here
    const loading = ref(false); // Loading state
    const error = ref(null); // Error state

    const fetchData = async (optionsHook = {}) => {
        loading.value = true; // Start loading
        error.value = null; // Reset the error
        options = { ...options, ...optionsHook};
        try {
            console.log(baseApiUrl + "####################")
            // Make the request using Axios
            const response = await axios({
                url: baseApiUrl + url,
                timeout: 600000, // 600 seconds = 10 minutes
                method: options.method || 'GET', // Default method is GET
                ...options, // Spread other axios options like headers, data, params, etc.
            });
            data.value = response.data; // Set the data from the response
        } catch (err) {
            error.value = err.message || 'Unknown error'; // Catch and set the error message
        } finally {
            loading.value = false; // Stop loading
        }
    };

    // Fetch data when this hook is called
    fetchData();

    // Return reactive properties so you can use them in the component
    return {
        data,
        loading,
        error,
        fetchData, // Expose the fetchData method if you want to manually re-fetch
    };
}
