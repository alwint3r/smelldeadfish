import { render } from "preact";
import { App } from "./App";
import "./styles/theme.css";
import "./styles/app.css";

const root = document.getElementById("app");
if (root) {
  render(<App />, root);
}
