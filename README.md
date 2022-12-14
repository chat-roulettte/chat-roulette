# Chat Roulette

_Chat Roulette for Slack is an open-source chat-roulette app for Slack. A no-frills, self-hosted, free alternative to the popular [Donut](https://www.donut.com/) app._

[![License](https://img.shields.io/badge/License-AGPL-orange.svg)](https://www.gnu.org/licenses/agpl-3.0.en.html)
[![Go](https://img.shields.io/badge/Go-1.19-blue.svg)](#)
[![Go Report Card](https://img.shields.io/badge/go%20report-A%2B-brightgreen)](https://goreportcard.com/report/github.com/chat-roulettte/chat-roulette)


## What is Chat Roulette?

Chat Roulette helps you stay connected to your Slack community by introducing you to other members on a regular cadence.

It works by inviting the `@chat-roulette-bot` to your Slack channel. The bot will pair members of the Slack channel every round (eg, every two weeks), giving participants enough time to meet for a video call before the start of the next chat-roulette round.

### Screenshots

*Click on the images to view full-screen.*

| ![App Home](./docs/images/screenshots/app-home.png) | ![Intro Message](./docs/images/screenshots/intro-message.png) | ![Onboarding](./docs/images/screenshots/onboarding.png) | ![Onboarding](./docs/images/screenshots/onboarding-location.png) | ![Calendly](./docs/images/screenshots/calendly.png) |
| :--------: | :---------: | :-----: | :-----: | :-----: |
| __App Home__ | __Intro Message__  | __Onboarding__ | __Onboarding Location__ | __Calendly Integration__ |
| ![Match](./docs/images/screenshots/match.png) |![UI](./docs/images/screenshots/ui.png) | ![History](./docs/images/screenshots/history.png) | ![Channel Settings](./docs/images/screenshots/channel-settings.png) | ![Profile Settings](./docs/images/screenshots/profile-settings.png) |
| __Match__ | __UI__  | __History__ | __Channel Settings__ | __Profile Settings__ |


## Deployment

See the [deployment guide](./docs/deployment.md) for how to run the app on [fly.io](https://fly.io/) or similar platforms.

## Configuration

To customize the configuration for the app, see [configuration.md](./docs/configuration.md).

## Contributing

_Chat Roulette for Slack_ is free, open-source software licensed under AGPLv3.

We encourage the following contributions at this time: user feedback, documentation, and bug reports.

To get started, take a look at [CONTRIBUTING.md](./CONTRIBUTING.md) and the [development guide](./docs/development.md).

## Acknowledgements

### Contributors

_Chat Roulette for Slack_ was made possible thanks to the work of the following contributors:

<table>
  <tbody>
    <tr>
      <td align="center"><a href="https://github.com/bincyber"><img src="https://avatars.githubusercontent.com/u/25866883?v=4?s=100" width="100px;" alt="Ali"/><br /><sub><b>Ali</b></td>
      <td align="center"><a href="https://github.com/AhmedARmohamed"><img src="https://avatars.githubusercontent.com/u/44018986?v=4?s=100" width="100px;" alt="Ahmed Mohamed"/><br /><sub><b>Ahmed Mohamed</b></td>
      <td align="center"><a href="https://github.com/Mohamed-C0DE"><img src="https://avatars.githubusercontent.com/u/60451644?v=4?s=100" width="100px;" alt="Mohamed Ali"/><br /><sub><b>Mohamed Ali</b></td>
      <td align="center"><a href="https://github.com/moabukar"><img src="https://avatars.githubusercontent.com/u/76791648?v=4?s=100" width="100px;" alt="Mohamed Abukar"/><br /><sub><b>Mohamed Abukar</b></td>
    </tr>
  </tbody>
</table>

### Libraries

_Chat Roulette for Slack_ was built using the Go libraries listed in [go.mod](go.mod).

## License

_Chat Roulette for Slack_ is distributed under [AGPL-3.0](LICENSE).
