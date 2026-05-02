export default class SearchDropDown extends HTMLElement {
    css = `
  :host {
    position: relative;
    width: 100%;
  }

  .dropdown-content {
  display: none;
  position: absolute;
  background-color: #fff;
  min-width: 100%;
  overflow: auto;
  border: 1px solid #ddd;
  z-index: 1;
  height: 500px;
}

.dropdown-content a {
  color: black;
  padding: 12px 16px;
  text-decoration: none;
  display: block;
}

.dropdown-content a:hover {background-color: #ddd;}

.show {
  display: inline-block;
}
`;

    constructor() {
        super();
        this.attachShadow({ mode: 'open' });
        this.render();
    }

    toggleDropDown() {
        this.shadowRoot.getElementById('custom-search-drop-down').classList.toggle('show');
    }

    template() {
        return `<div id="custom-search-drop-down" class="dropdown-content">
    <a href="#about">About</a>
    <a href="#base">Base</a>
    <a href="#blog">Blog</a>
    <a href="#contact">Contact</a>
    <a href="#custom">Custom</a>
    <a href="#support">Support</a>
    <a href="#tools">Tools</a>
    <a href="#about">About</a>
    <a href="#about">About</a>
    <a href="#about">About</a>
    <a href="#about">About</a>
    <a href="#base">Base</a>
    <a href="#blog">Blog</a>
    <a href="#contact">Contact</a>
    <a href="#custom">Custom</a>
    <a href="#support">Support</a>
    <a href="#tools">Tools</a>
    <a href="#base">Base</a>
    <a href="#blog">Blog</a>
    <a href="#contact">Contact</a>
    <a href="#custom">Custom</a>
    <a href="#support">Support</a>
    <a href="#tools">Tools</a>
    <a href="#base">Base</a>
    <a href="#blog">Blog</a>
    <a href="#contact">Contact</a>
    <a href="#custom">Custom</a>
    <a href="#support">Support</a>
    <a href="#tools">Tools</a>
    <a href="#base">Base</a>
    <a href="#blog">Blog</a>
    <a href="#contact">Contact</a>
    <a href="#custom">Custom</a>
    <a href="#support">Support</a>
    <a href="#tools">Tools</a>
  </div>`;
    }

    handleRemoveElements() {
        const elements = [this.shadowRoot.getElementById('custom-search-drop-down')];
        const elementLen = elements.length;
        for (let index = 0; index < elementLen; index++) {
            const element = elements[index];
            if (element) {
                element.remove();
            }
        }
    }

    render() {
        this.shadowRoot.innerHTML = `
    <style>${this.css.trim()}</style>
    ${this.template().trim()}
  `;
    }
}
