<script>
  let out = $state("waiting for host");

  function show(err) {
    const code = err && err.code ? err.code + ": " : "";
    const message = err && err.message ? err.message : String(err);
    out = code + message;
  }
</script>

<main>
  <h1>__NAME__</h1>
  <p>
    <button
      type="button"
      onclick={async () => {
        try {
          out = String(await window.__XGOEF__.invoke("greet", { name: "__NAME__" }));
        } catch (err) {
          show(err);
        }
      }}
    >
      invoke greet
    </button>
    <button
      type="button"
      onclick={async () => {
        try {
          out = String(await window.__XGOEF__.plugins.fs.readText("web/note.txt"));
        } catch (err) {
          show(err);
        }
      }}
    >
      read note
    </button>
    <button
      type="button"
      onclick={async () => {
        try {
          out = "platform: " + (await window.__XGOEF__.plugins.os.platform());
        } catch (err) {
          show(err);
        }
      }}
    >
      os platform
    </button>
  </p>
  <div>{out}</div>
</main>
