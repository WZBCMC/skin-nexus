// lifecycle-log: observe auth and account lifecycle events.
// Events are fire-and-forget: returning a value or throwing never changes
// the host response (fail-open).
skin.hooks.onUserLifecycle(function (e) {
  skin.log("lifecycle", e.kind, "uid=" + e.userId);
});

skin.hooks.onAuthPhase(function (e) {
  skin.log("auth", e.phase, e.player, e.uuid);
});
