const inputs = document.querySelectorAll(".input");

function addcl() {
  let parent = this.parentNode.parentNode;
  parent.classList.add("focus");
}

function remcl() {
  let parent = this.parentNode.parentNode;
  if (this.value == "") {
    parent.classList.remove("focus");
  }
}

inputs.forEach((input) => {
  input.addEventListener("focus", addcl);
  input.addEventListener("blur", remcl);
});

// Event when button with id of 'mobile-user-icon' is clicked
document.getElementById("mobile-user-icon").addEventListener("click", () => {
  const element = document.getElementById("mobile-dropdown");
  if (element.classList.contains("hidden")) {
    element.classList.remove("hidden");
  } else {
    element.classList.add("hidden");
  }
});

// Event when button with id of 'user-icon' is clicked
document.getElementById("user-icon").addEventListener("click", () => {
  const element = document.getElementById("dropdown");
  if (element.classList.contains("hidden")) {
    element.classList.remove("hidden");
  } else {
    element.classList.add("hidden");
  }
});

// Open Mobile Sidebar
document.getElementById("hamburger-menu").addEventListener("click", () => {
  document.getElementById("mobile-sidebar").classList.remove("hidden");
});

// Close Mobile Sidebar
document.getElementById("close-btn").addEventListener("click", () => {
  document.getElementById("mobile-sidebar").classList.add("hidden");
});
