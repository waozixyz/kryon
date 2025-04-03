// impl/js/main.js
import { KrbParser } from './krb-parser.js';
import { renderKrb } from './krb-renderer.js';
// Import MUST work - ensure example-manifest.js exists here after build
import { exampleFiles } from './example-manifest.js';

// --- Event Handlers (Keep as they are) ---
window.handleButtonClick = function(event) { console.log("Click!", event.target); alert('Click!'); };
window.handleImageHover = function(event) { console.log("Hover!", event.type, event.target); };
window.handleInputChange = function(event) { console.log("Input!", event.target.value); };
window.uiCallbacks = { nested: { buttonPress: (event) => { console.log("Nested Press!", event.target); alert('Nested!'); } } };
// --- End Event Handlers ---


// --- UI Element References ---
const fileInput = document.getElementById('krbFileInput');
const exampleSelector = document.getElementById('exampleSelector');
const renderTarget = document.getElementById('kryon-root');
const statusDiv = document.getElementById('status');

// --- Core Logic (processKrbBuffer & loadAndRender - keep previous version) ---

function processKrbBuffer(arrayBuffer, fileName) {
    if (!renderTarget || !statusDiv) { console.error("Missing UI elements"); return; }
    statusDiv.textContent = `Parsing ${fileName}...`;
    renderTarget.innerHTML = '<p>Parsing...</p>';
    const parser = new KrbParser();
    let krbDocument = null;
    try {
        krbDocument = parser.parse(arrayBuffer);
    } catch (parseError) {
        console.error(`Parsing Error (${fileName}):`, parseError);
        statusDiv.textContent = `Error parsing ${fileName}: ${parseError.message}`;
        renderTarget.innerHTML = `<p style="color: red;">Parsing Error. Check console.</p>`;
        return;
    }
    if (krbDocument) {
        statusDiv.textContent = `Rendering ${fileName}...`;
        try {
            renderKrb(krbDocument, renderTarget);
            statusDiv.textContent = `Rendered ${fileName} (v${krbDocument.versionMajor}.${krbDocument.versionMinor}) successfully.`;
        } catch (renderError) {
            console.error(`Rendering Error (${fileName}):`, renderError);
            statusDiv.textContent = `Error rendering ${fileName}: ${renderError.message}`;
            renderTarget.innerHTML = `<p style="color: red;">Rendering Error: ${renderError.message}. Check console.</p>`;
        }
    } else {
         if (!statusDiv.textContent.startsWith("Error parsing")) statusDiv.textContent = `Failed to parse ${fileName}. Check console.`;
         if (!renderTarget.innerHTML.startsWith("<p style=\"color: red;\">")) renderTarget.innerHTML = `<p style="color: red;">Parsing failed. Check console.</p>`;
    }
}

function loadAndRender(source) {
     if (!renderTarget || !statusDiv) { console.error("Missing UI elements"); return; }
    if (source instanceof File) {
        const file = source;
        if (!file.name.toLowerCase().endsWith('.krb')) {
            statusDiv.textContent = 'Error: Please select a .krb file.';
            alert('Error: Please select a .krb file.');
            return;
        }
        statusDiv.textContent = `Loading ${file.name}...`;
        renderTarget.innerHTML = '<p>Loading...</p>';
        const reader = new FileReader();
        reader.onload = (e) => processKrbBuffer(e.target.result, file.name);
        reader.onerror = (e) => {
            console.error("File Reading Error:", e);
            statusDiv.textContent = `Error reading file: ${file.name}`;
            renderTarget.innerHTML = `<p style="color: red;">Error reading file.</p>`;
        };
        reader.readAsArrayBuffer(file);
    } else if (typeof source === 'string' && source !== "") {
        const filename = source;
        const url = `examples/${filename}`; // Relative to index.html in dist/
        statusDiv.textContent = `Fetching ${filename}...`;
        renderTarget.innerHTML = '<p>Fetching...</p>';
        fetch(url)
            .then(response => {
                if (!response.ok) throw new Error(`HTTP ${response.status} fetching ${url}`);
                return response.arrayBuffer();
            })
            .then(arrayBuffer => processKrbBuffer(arrayBuffer, filename))
            .catch(error => {
                console.error(`Error fetching example ${filename}:`, error);
                statusDiv.textContent = `Error fetching ${filename}: ${error.message}`;
                renderTarget.innerHTML = `<p style="color: red;">Could not fetch example: ${error.message}.</p>`;
            });
    } else {
         console.warn("loadAndRender called with invalid source:", source);
         statusDiv.textContent = 'Invalid source.';
    }
}

/** Populates the example dropdown */
function populateExamplesDropdown() {
    // Ensure selector exists before proceeding
    if (!exampleSelector) {
         console.error("Example selector dropdown not found in DOM.");
         return;
    }
    // Clear existing options except the placeholder
    const placeholder = exampleSelector.options[0] || document.createElement('option');
    placeholder.value = "";
    placeholder.textContent = "Select Example...";
    exampleSelector.innerHTML = ''; // Clear all
    exampleSelector.appendChild(placeholder); // Add placeholder back

    // Check if exampleFiles array exists and has content
    if (!exampleFiles || !Array.isArray(exampleFiles)) {
         placeholder.textContent = "Manifest Error";
         exampleSelector.disabled = true;
         console.error("exampleFiles is not loaded or not an array. Check import and example-manifest.js generation.");
         return;
    }
    if (exampleFiles.length === 0) {
        placeholder.textContent = "No examples found";
        exampleSelector.disabled = true;
        console.warn("No example KRB files listed in example-manifest.js");
        return;
    }

    // Populate with examples
    exampleFiles.forEach(filename => {
        const option = document.createElement('option');
        option.value = filename;
        option.textContent = filename;
        exampleSelector.appendChild(option);
    });
    exampleSelector.disabled = false;
    placeholder.textContent = "Select Example..."; // Ensure placeholder text is reset
     console.log(`Populated dropdown with ${exampleFiles.length} examples.`);
}

// --- Initialization and Event Listeners ---
function initialize() {
    // Check essential elements exist
    if (!fileInput || !exampleSelector || !renderTarget || !statusDiv) {
        console.error('One or more required UI elements are missing (fileInput, exampleSelector, kryon-root, status). Initialization failed.');
        if (statusDiv) statusDiv.textContent = 'Error: UI elements missing!';
        // Disable inputs if essential elements are missing
        if (fileInput) fileInput.disabled = true;
        if (exampleSelector) exampleSelector.disabled = true;
        return; // Stop initialization
    }

    console.log("Initializing UI..."); // Log start of init

    // Populate the dropdown (crucial fix)
    try {
        populateExamplesDropdown();
    } catch(e) {
        console.error("Error during populateExamplesDropdown:", e);
        if (exampleSelector) {
             exampleSelector.innerHTML = '<option value="">Error loading examples</option>';
             exampleSelector.disabled = true;
        }
    }


    // Listener for the file input
    fileInput.addEventListener('change', (event) => {
        const file = event.target.files[0];
        if (file) {
             exampleSelector.value = ""; // Reset dropdown
            loadAndRender(file);
        }
    });

    // Listener for the example dropdown
    exampleSelector.addEventListener('change', (event) => {
        const selectedFilename = event.target.value;
        if (selectedFilename) {
             fileInput.value = ''; // Clear file input
            loadAndRender(selectedFilename);
        }
    });

    statusDiv.textContent = 'Ready. Select an example or load a local file.';
    console.log("UI Initialized.");

    // Optional: Load default after initialization is complete
    // const defaultExample = 'hello_world.krb';
    // if (exampleFiles && exampleFiles.includes(defaultExample)) {
    //      console.log(`Loading default example: ${defaultExample}`);
    //      exampleSelector.value = defaultExample;
    //      loadAndRender(defaultExample);
    // }
}

// Run initialization safely after DOM is likely loaded
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initialize);
} else {
    initialize(); // DOMContentLoaded has already fired
}