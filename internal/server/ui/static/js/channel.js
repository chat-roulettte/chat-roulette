
// Submit form to /v1/channel API endpoint
document
  .getElementById("channel-settings-form")
  .addEventListener("submit", async function (event) {
    event.preventDefault();

    const form = event.currentTarget;

    var data = new FormData(form);

    // Extract channel_id from the route
    const channel_id = window.location.href.split("/").pop();

    // Get the date for the next chat-roulette round (must be in UTC)
    let next_round = new Date(data.get("next-round"));
    const hour = Number(data.get("hour"));

    next_round.setUTCHours(hour);

    // Request body for the updateChannel API endpoint
    let body = {
      channel_id: channel_id,
      interval: data.get("interval"),
      weekday: data.get("weekday"),
      hour: Number(data.get("hour")),
      next_round: next_round,
      connection_mode: data.get("connection-mode"),
    };

    let response = await fetch(form.action, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(body),
    });

    if (!response.ok) {
      // Flash error alert
      error = await response.json();

      p = document.getElementById("error-alert-text");
      p.textContent = error.error;

      div = document.getElementById("error-alert");
      div.classList.remove("hidden");

      setTimeout(function () {
        div.classList.add("hidden");
      }, 5000); // 5 seconds

      throw new Error("failed to update channel's settings");
    }

    // Flash the alert, then redirect back to /profile.
    // Note: the backend is eventually consistent because the UPDATE_CHANNEL task
    // will be completed asynchronously.
    document.getElementById("success-alert").classList.remove("hidden");

    setTimeout(function () {
      window.location.replace("/profile");
    }, 3000); // 3 seconds
  });