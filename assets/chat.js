$(document).ready(function() {
	var protocol = "ws:";
	if (window.location.protocol == "https:") {
		protocol = "wss:"
	}
	var ws = new WebSocket(protocol+"//"+window.location.host+"/chat");
	ws.onmessage = function(event) {
		$("#messages").append($("<p>").text(event.data))
	};

	$("#input").keypress(function(event) {
		if (event.which == 13) {
			ws.send($("#input").val())
			$("#input").val("")
		}
	});
});
