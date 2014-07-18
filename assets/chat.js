$(document).ready(function() {
	$("#messages").css("padding-top", $("#controls").outerHeight()+8+"px")

	var protocol = "ws:";
	if (window.location.protocol == "https:") {
		protocol = "wss:"
	}
	var ws = new WebSocket(protocol+"//"+window.location.host+"/chat");
	ws.onopen = function(event) {
		$("#status").html(":)")
	};
	ws.onclose = function(event) {
		$("#status").html(":(")
	};
	ws.onmessage = function(event) {
		$("#messages").append($("<p>").text(event.data))
		window.scrollTo(window.scrollX, $("html").height())
	};
	ws.onerror = function(event) {
		$("#messages").append($("<p>").addClass("error").text(event.data))
		window.scrollTo(window.scrollX, $("html").height())
	}

	$("#input").keypress(function(event) {
		if (event.which == 13) {
			ws.send($("#input").val())
			$("#input").val("")
		}
	});
});
