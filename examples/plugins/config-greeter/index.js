// Config + public route: GET /api/plugins/config-greeter/hello
skin.route("GET", "/hello", function () {
  var message = skin.config.get("message") || "Hello from a plugin";
  var repeat = Number(skin.config.get("repeat") || 1);
  var text = "";
  for (var i = 0; i < repeat; i++) text += message + " ";
  return { message: text.trim(), repeat: repeat };
});
