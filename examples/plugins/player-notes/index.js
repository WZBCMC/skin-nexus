// Namespaced data + session routes. The host enforces the session tier,
// CSRF on POST, quotas, and exposes only the declared collection.
var notes = skin.store.collection("notes");

skin.store.onInstall(function () {
  // schemaVersion 1: no data transform needed, just prove the lifecycle ran.
  skin.log("player-notes installed");
});

skin.route("GET", "/notes", function (r) {
  return { items: notes.list({ limit: 100 }), user: r.user.id };
});

skin.route("POST", "/notes", function (r) {
  var text = String((r.body && r.body.text) || "").trim();
  if (!text) return { status: 400, body: { error: "text required" } };
  var id = String(Date.now());
  notes.put(id, { text: text, createdAt: new Date().toISOString() });
  return { id: id };
});
