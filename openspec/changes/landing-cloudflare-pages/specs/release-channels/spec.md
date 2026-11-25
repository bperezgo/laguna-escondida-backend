## ADDED Requirements

### Requirement: A landing tag publishes the site
A `landing/v*` tag SHALL publish the built site to the production deployment of the landing's Cloudflare Pages project, and SHALL attach the same build to a GitHub Release named after the tag as a zip with its sha256. The site SHALL be published only after the landing check, test and build pass. The build SHALL take its site address, backend address and optional tracking and contact settings from repository configuration, never from a value written in the workflow.

#### Scenario: Tag publishes the site
- **WHEN** a `landing/v1.0.0` tag is pushed and check, test and build pass
- **THEN** the site served at the Pages production address is the one built from that tag's commit
- **AND** a GitHub Release `landing/v1.0.0` carries `landing-site-v1.0.0.zip` and its sha256 file, built in the same run

#### Scenario: Failing check publishes nothing
- **WHEN** a `landing/v*` tag is pushed and check, test or build fails
- **THEN** the site served at the Pages production address is unchanged
- **AND** no GitHub Release is created for the tag

#### Scenario: Missing backend address publishes nothing
- **WHEN** a `landing/v*` tag is pushed while the backend address setting is empty
- **THEN** the build fails naming the missing setting
- **AND** the site served at the Pages production address is unchanged

#### Scenario: Form posts to the configured backend
- **WHEN** the backend address setting is `https://api.laguna-escondida.com` and a tag is published
- **THEN** the published demo form sends its request to `https://api.laguna-escondida.com/api/public/leads`

### Requirement: An earlier landing release can be republished
An operator SHALL be able to republish the site of an existing `landing/v*` tag to the production deployment by hand, without creating a new tag or a new GitHub Release.

#### Scenario: Rollback to an earlier tag
- **WHEN** `landing/v1.1.0` is live and an operator republishes `landing/v1.0.0`
- **THEN** the site served at the Pages production address is the one built from `landing/v1.0.0`
- **AND** the existing `landing/v1.0.0` release is left as it was
