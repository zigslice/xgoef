import "./style.css";

const app = document.querySelector("#app");
app.innerHTML = `
  <h1>__NAME__</h1>
  <button id="btn" type="button">invoke greet</button>
  <button id="btn-fs" type="button">read note</button>
  <button id="btn-os" type="button">os platform</button>
  <div id="out">waiting for host</div>
`;

const out = document.getElementById("out");

function show(err) {
  const code = err && err.code ? err.code + ": " : "";
  const message = err && err.message ? err.message : String(err);
  out.textContent = code + message;
}

document.getElementById("btn").onclick = async () => {
  try {
    out.textContent = String(await window.__XGOEF__.invoke("greet", { name: "__NAME__" }));
  } catch (err) {
    show(err);
  }
};

document.getElementById("btn-fs").onclick = async () => {
  try {
    out.textContent = String(await window.__XGOEF__.plugins.fs.readText("web/note.txt"));
  } catch (err) {
    show(err);
  }
};

document.getElementById("btn-os").onclick = async () => {
  try {
    out.textContent = "platform: " + (await window.__XGOEF__.plugins.os.platform());
  } catch (err) {
    show(err);
  }
};
