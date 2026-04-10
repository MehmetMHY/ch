/**
 * YouTube Facade logic for Ch CLI landing page
 */

export function initYouTubeFacades() {
  document.querySelectorAll(".youtube-facade").forEach((facade) => {
    facade.addEventListener("click", function () {
      const id = this.dataset.videoId;
      const iframe = document.createElement("iframe");
      iframe.className = "demo-video";
      iframe.src =
        "https://www.youtube-nocookie.com/embed/" + id + "?autoplay=1";
      iframe.title = "Ch Demo Video";
      iframe.setAttribute("frameborder", "0");
      iframe.setAttribute(
        "allow",
        "accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share",
      );
      iframe.setAttribute("referrerpolicy", "strict-origin-when-cross-origin");
      iframe.setAttribute("allowfullscreen", "");
      this.replaceWith(iframe);
    });
  });
}
