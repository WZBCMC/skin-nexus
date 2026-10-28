// launcher-api: SkinNexus Launcher API v1.
// Thin transport mapping; every ownership/permission check lives in the host
// account bridge (skin.account/players/textures/library).

var API_VERSION = 1;
var MAX_UPLOAD_BYTES = 2 * 1024 * 1024;
var MAX_LIBRARY_PAGE = 50;

function fail(e) {
  var code = (e && e.code) || "internal";
  var message = (e && e.message) || "internal error";
  var status = 500;
  if (code === "player.not_found" || code === "library.not_found") status = 404;
  else if (code === "player.limit" || code === "player.name_taken") status = 409;
  else if (code === "player.name_invalid" || code === "texture.invalid" || code === "texture.type_mismatch") status = 400;
  else if (code === "texture.too_large") status = 413;
  else if (code === "texture.not_owned" || code === "account.forbidden") status = 403;
  else if (code === "account.required") status = 401;
  else if (code === "account.unavailable") status = 503;
  return { status: status, body: { error: code, message: message } };
}

function ok(res, status) {
  if (!res || res.ok !== true) return fail(res && res.error);
  return { status: status || 200, body: res.data };
}

function pidOf(r) {
  var pid = Number(r.params && r.params.pid);
  return pid > 0 ? pid : 0;
}

function tidOf(r) {
  var tid = Number(r.params && r.params.tid);
  return tid > 0 ? tid : 0;
}

function nameOf(body) {
  return String((body && body.name) || "").trim();
}

function setTexture(r, kind) {
  var pid = pidOf(r);
  if (!pid) return fail({ code: "player.not_found", message: "invalid player id" });
  var body = r.body;
  if (body && typeof body === "object" && body.textureId !== undefined) {
    return ok(skin.players.setTexture(pid, kind, Number(body.textureId)));
  }
  if (typeof body !== "string" || !body) {
    return fail({ code: "texture.invalid", message: "PNG body or textureId required" });
  }
  var model = kind === "skin" ? String((r.query && r.query.model) || "default") : "";
  var up = skin.textures.uploadBase64(kind, model, "", body);
  if (!up.ok) return fail(up.error);
  return ok(skin.players.setTexture(pid, kind, up.data.textureId));
}

skin.route("GET", "/v1/meta", function () {
  return {
    status: 200,
    body: {
      apiVersion: API_VERSION,
      features: {
        bearer: true,
        idempotentCreate: true,
        players: true,
        textureUpload: true,
        cape: true,
        library: true,
        libraryImport: true
      },
      models: ["default", "slim"],
      limits: { maxUploadBytes: MAX_UPLOAD_BYTES, libraryPerPage: MAX_LIBRARY_PAGE }
    }
  };
});

skin.route("GET", "/v1/account", function () {
  return ok(skin.account.get());
});

skin.route("GET", "/v1/players", function () {
  var res = skin.players.list();
  if (!res.ok) return fail(res.error);
  return { status: 200, body: { players: res.data } };
});

skin.route("GET", "/v1/players/{pid}", function (r) {
  return ok(skin.players.get(pidOf(r)));
});

skin.route("POST", "/v1/players", function (r) {
  var name = nameOf(r.body);
  var res = skin.players.create(name);
  if (!res.ok) return fail(res.error);
  return { status: res.created ? 201 : 200, body: res.data };
});

skin.route("PATCH", "/v1/players/{pid}", function (r) {
  var res = skin.players.rename(pidOf(r), nameOf(r.body));
  if (!res.ok) return fail(res.error);
  var body = res.data;
  body.tokensInvalidated = !!res.tokensInvalidated;
  return { status: 200, body: body };
});

skin.route("DELETE", "/v1/players/{pid}", function (r) {
  var res = skin.players.delete(pidOf(r));
  if (!res.ok) return fail(res.error);
  return { status: 200, body: { deleted: true } };
});

skin.route("PUT", "/v1/players/{pid}/skin", function (r) { return setTexture(r, "skin"); });
skin.route("PUT", "/v1/players/{pid}/cape", function (r) { return setTexture(r, "cape"); });

skin.route("DELETE", "/v1/players/{pid}/skin", function (r) {
  return ok(skin.players.setTexture(pidOf(r), "skin", 0));
});
skin.route("DELETE", "/v1/players/{pid}/cape", function (r) {
  return ok(skin.players.setTexture(pidOf(r), "cape", 0));
});

skin.route("GET", "/v1/textures", function () {
  var res = skin.textures.list();
  if (!res.ok) return fail(res.error);
  return { status: 200, body: { textures: res.data || [] } };
});

skin.route("POST", "/v1/textures", function (r) {
  var typ = String((r.query && r.query.type) || "skin");
  var model = String((r.query && r.query.model) || "");
  var desc = String((r.query && r.query.description) || "");
  var res = skin.textures.uploadBase64(typ, model, desc, String(r.body || ""));
  if (!res.ok) return fail(res.error);
  return { status: res.created ? 201 : 200, body: res.data };
});

skin.route("GET", "/v1/library", function (r) {
  var q = r.query || {};
  var res = skin.library.list({
    type: String(q.type || q.filter || ""),
    q: String(q.q || q.keyword || ""),
    sort: String(q.sort || "time"),
    page: Number(q.page || 1),
    perPage: Number(q.per_page || q.perPage || 20)
  });
  return ok(res);
});

skin.route("GET", "/v1/library/{tid}", function (r) {
  return ok(skin.library.get(tidOf(r)));
});

skin.route("POST", "/v1/library/{tid}/import", function (r) {
  var res = skin.textures.importLibrary(tidOf(r));
  if (!res.ok) return fail(res.error);
  return { status: res.created ? 201 : 200, body: res.data };
});
