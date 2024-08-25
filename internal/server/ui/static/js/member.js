// Submit form to /v1/member API endpoint
document
  .getElementById("member-profile-form")
  .addEventListener("submit", async function (event) {
    event.preventDefault();

    const form = event.currentTarget;

    var data = new FormData(form);

    // Get the Slack user's ID from this element
    let user = document.getElementById("slack-user-id");

    // Extract channel_id from the route
    channel_id = window.location.href.split("/").pop();

    // Request body for the updateMember API endpoint
    let body = {
      channel_id: channel_id,
      user_id: user.dataset.slackUserId,
      country: data.get("location-country"),
      city: data.get("location-city"),
      timezone: data.get("location-timezone"),
      profile_type: data.get("profile-type"),
      profile_link: data.get("profile-link"),
      calendly_link: data.get("calendly"),
      is_active: data.get("is-active") === "true",
      has_gender_preference: data.get("has-gender-preference") === "true",
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

      throw new Error("failed to update member's profile settings");
    }

    // Flash the alert, then redirect back to /profile.
    // Note: the backend is eventually consistent because the UPDATE_MEMBER task
    // will be completed asynchronously.
    document.getElementById("success-alert").classList.remove("hidden");

    setTimeout(function () {
      window.location.replace("/profile");
    }, 3000); // 3 seconds
  });

// Populate timezones select menu based on currently selected Country
document
  .getElementById("select-country")
  .addEventListener("change", async function (event) {
    timezoneMenu = document.getElementById("select-timezone");

    // First, reset all existing options
    while (timezoneMenu.options.length > 0) {
      timezoneMenu.remove(0);
    }

    // Retrieve the timezones for the specified country
    url = "/v1/timezones/" + event.target.value;

    let response = await fetch(url, {
      method: "GET",
      headers: {
        Accept: "application/json",
      },
    });

    if (!response.ok) {
      // TODO: better error handling
      console.error(response);
      throw new Error("failed to retrieve timezones for specified country");
    }

    // Set zones as options
    data = await response.json();

    for (const zone of data.Zones) {
      opt = new Option(zone, zone);
      timezoneMenu.add(opt, undefined);
    }
  });
