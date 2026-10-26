<script setup>
import { ref } from "vue";

const out = ref("waiting for host");

function show(err) {
  const code = err && err.code ? err.code + ": " : "";
  const message = err && err.message ? err.message : String(err);
  out.value = code + message;
}

async function greet() {
  try {
    out.value = String(await window.__XGOEF__.invoke("greet", { name: "__NAME__" }));
  } catch (err) {
    show(err);
  }
}

async function readNote() {
  try {
    out.value = String(await window.__XGOEF__.plugins.fs.readText("web/note.txt"));
  } catch (err) {
    show(err);
  }
}

async function platform() {
  try {
    out.value = "platform: " + (await window.__XGOEF__.plugins.os.platform());
  } catch (err) {
    show(err);
  }
}
</script>

<template>
  <main>
    <h1>__NAME__</h1>
    <p>
      <button type="button" @click="greet">invoke greet</button>
      <button type="button" @click="readNote">read note</button>
      <button type="button" @click="platform">os platform</button>
    </p>
    <div>{{ out }}</div>
  </main>
</template>
