
# Responsible Disclosure Policy

Flow was built from the ground up with security in mind. Our code, infrastructure, and development methodology helps us keep our users safe.

We really appreciate the community's help. Responsible disclosure of vulnerabilities helps to maintain the security and privacy of everyone.

If you care about making a difference, please follow the guidelines below.

# **Guidelines For Responsible Disclosure**

We ask that all researchers adhere to these guidelines.

## **Rules of Engagement**

- Make every effort to avoid unauthorized access, use, and disclosure of personal information.
- Avoid actions which could impact user experience, disrupt production systems, change, or destroy data during security testing.
- Don’t perform any attack that is intended to cause Denial of Service to the network, hosts, or services on any port or using any protocol.
- Use our provided communication channels to securely report vulnerability information to us.
- Keep information about any bug or vulnerability you discover confidential between us until we publicly disclose it.
- Please don’t use scanners to crawl us and hammer endpoints. They’re noisy and we already do this. If you find anything this way, we have likely already identified it.
- Never attempt non-technical attacks such as social engineering, phishing, or physical attacks against our employees, users, or infrastructure.

## **In Scope URIs**

Be careful that you're looking at domains and systems that belong to us and not someone else. When in doubt, please ask us. Maybe ask us anyway.

Bottom line, we suggest that you limit your testing to infrastructure that is clearly ours.

## **Out of Scope URIs**

The following base URIs are explicitly out of scope:

- None

## **Things Not To Do**

In the interests of your safety, our safety, and for our customers, the following test types are prohibited:

- Physical testing such as office and data-centre access (e.g. open doors, tailgating, card reader attacks, physically destructive testing)
- Social engineering (e.g. phishing, vishing)
- Testing of applications or systems NOT covered by the ‘In Scope’ section, or that are explicitly out of scope.
- Network level Denial of Service (DoS/DDoS) attacks

## **Sensitive Data**

In the interests of protecting privacy, we never want to receive:

- Personally identifiable information (PII)
- Payment card (e.g. credit card) data
- Financial information (e.g. bank records)
- Health or medical information
- Accessed or cracked credentials in cleartext

## **Our Commitment To You**

If you follow these guidelines when researching and reporting an issue to us, we commit to:

- Not send lawyers after you related to your research under this policy;
- Work with you to understand and resolve any issues within a reasonable timeframe, including an initial confirmation of your report within 72 hours of submission; and
- At a minimum, we will recognize your contribution in our Disclosure Acknowledgements if you are the first to report the issue and we make a code or configuration change based on the issue.

## **Disclosure Acknowledgements**

We're happy to acknowledge contributors. Security acknowledgements can be found here.

## Rewards

We run closed bug bounty programs, but beyond that we also pay out rewards, once per eligible bug, to the first responsibly disclosing third party.  Rewards are based on the seriousness of the bug, but the minimum is $100 and we have and are willing to pay $5,000 or more at our sole discretion.

### **Elligibility**

To qualify, the bug must fall within our scope and rules and meet the following criteria:

1. **Previously unknown** - When reported, we must not have already known of the issue, either by internal discovery or separate disclosure.
2. **Material impact** - Demonstrable exploitability where, if exploited, the bug would materially affect the confidentiality, integrity, or availability of our services.
3. **Requires action** - The bug requires some mitigation.  It is both valid and actionable.

## **Reporting Security Findings To Us**

Reports are welcome! Please definitely reach out to us if you have a security concern.

We prefer you to please send us an email: security@onflow.org

Note: If you believe you may have found a security vulnerability in our open source repos, to be on the safe side, do NOT open a public issue.

We encourage you to encrypt the information you send us using our PGP key at [keys.openpgp.org/security@onflow.org](https://keys.openpgp.org/vks/v1/by-fingerprint/AE3264F330AB51F7DBC52C400BB5D3D7516D168C)

Please include the following details with your report:

- A description of the location and potential impact of the finding(s);
- A detailed description of the steps required to reproduce the issue; and
- Any POC scripts, screenshots, and compressed screen captures, where feasible.
