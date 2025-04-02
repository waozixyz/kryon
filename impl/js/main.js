import { KrbParser } from './krb-parser.js';
import { renderKrb } from './krb-renderer.js';

// --- Placeholder Event Handlers (Define functions expected by KRB events) ---
// These MUST be globally accessible (window scope) for findCallback to work.
window.handleButtonClick = function(event) {
    console.log("Button Clicked!", event.target);
    alert(`Element ${event.target.id || event.target.dataset.krbIndex} clicked!`);
};

window.handleImageHover = function(event) {
    console.log("Image Hovered!", event.type, event.target);
    if (event.type === 'pointerenter') {
        event.target.style.filter = 'brightness(1.2)';
    } else if (event.type === 'pointerleave') {
        event.target.style.filter = 'none';
    }
};

window.handleInputChange = function(event) {
     console.log(`Input Changed (ID: ${event.target.id || 'N/A'}):`, event.target.value);
}

// --- File Loading and Rendering Logic ---
const fileInput = document.getElementById('krbFileInput');
const renderTarget = document.getElementById('kryon-root');
const statusDiv = document.getElementById('status');

function loadAndRender(file) {
    if (!file) {
        statusDiv.textContent = 'No file selected.';
        return;
    }
    if (!file.name.toLowerCase().endsWith('.krb')) {
        statusDiv.textContent = 'Error: Please select a .krb file.';
        alert('Error: Please select a .krb file.');
        return;
    }

    statusDiv.textContent = `Loading ${file.name}...`;
    renderTarget.innerHTML = '<p>Loading...</p>'; // Clear previous render

    const reader = new FileReader();

    reader.onload = function(e) {
        statusDiv.textContent = `Parsing ${file.name}...`;
        const arrayBuffer = e.target.result;
        const parser = new KrbParser();
        const krbDocument = parser.parse(arrayBuffer);

        if (krbDocument) {
            statusDiv.textContent = `Rendering ${file.name}...`;
            try {
                renderKrb(krbDocument, renderTarget);
                statusDiv.textContent = `Rendered ${file.name} successfully.`;
            } catch (renderError) {
                console.error("Rendering Error:", renderError);
                statusDiv.textContent = `Error rendering ${file.name}: ${renderError.message}`;
                renderTarget.innerHTML = `<p style="color: red;">Error during rendering. Check console.</p>`;
            }
        } else {
            statusDiv.textContent = `Failed to parse ${file.name}. Check console for errors.`;
            renderTarget.innerHTML = `<p style="color: red;">Error during parsing. Check console.</p>`;
        }
    };

    reader.onerror = function(e) {
        console.error("File Reading Error:", e);
        statusDiv.textContent = `Error reading file: ${file.name}`;
        renderTarget.innerHTML = `<p style="color: red;">Error reading file.</p>`;
    };

    reader.readAsArrayBuffer(file);
}

// --- Event Listener ---
if (fileInput) {
    fileInput.addEventListener('change', (event) => {
        const file = event.target.files[0];
        loadAndRender(file);
    });
    statusDiv.textContent = 'Select a .krb file to render.';
} else {
     statusDiv.textContent = 'File input not found.';
}

// --- Optional: Load a default file on page load ---
// Uncomment and set the path relative to the HTML file
/*
fetch('../examples/interface.krb') // Adjust path as needed
    .then(response => {
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        return response.arrayBuffer();
    })
    .then(arrayBuffer => {
         statusDiv.textContent = `Parsing default file...`;
         const parser = new KrbParser();
         const krbDocument = parser.parse(arrayBuffer);
         if (krbDocument) {
            statusDiv.textContent = `Rendering default file...`;
            renderKrb(krbDocument, renderTarget);
            statusDiv.textContent = `Rendered default file successfully.`;
         } else {
            statusDiv.textContent = `Failed to parse default file.`;
         }
    })
    .catch(error => {
        console.error("Error loading default KRB file:", error);
        statusDiv.textContent = 'Could not load default KRB file.';
         renderTarget.innerHTML = `<p style="color: orange;">Could not load default .krb file. Use the file selector.</p>`;
    });
*/