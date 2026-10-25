# Compatibility notes (verified 2026-10-25)

Discord's Gateway delivers events selected through intents; Guild Members is privileged and is requested only for onboarding. Interaction commands, buttons, select menus, and modals are supported through discordgo's interaction API. Components V2 requires the `IS_COMPONENTS_V2` message flag and removes legacy `content`/embeds for that message, so CommunityOS uses legacy-compatible component panels until the Go binding exposes every V2 object. Form types not supported by Discord modals must use explicit staged component interactions rather than being represented as a modal.

Discord rate limits are dynamic; clients must honor response headers rather than hard-coding limits. OCI availability is tenancy/region specific and must be checked by the operator before provisioning.

Sources: Discord Gateway, Interactions, Components, and Rate Limit official documentation; discordgo repository; OCI Always Free documentation.
