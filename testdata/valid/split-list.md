<!-- <atomdown version="1"/> -->
<!-- <atom id="J1BBCED5"/> -->
# Release Acceptance Criteria: Payment Retry

<!-- <atom-group id="KF53ASNE"> -->
<!-- <atom id="FAPWJSRC"/> -->
* A failed charge retries a maximum of three times.
<!-- <atom id="GPG5QA7A"/> -->
* Retries use exponential backoff starting at 30 seconds.
<!-- <atom id="DJBG7T1N"/> -->
* A retry never fires more than 24 hours after the original attempt.
<!-- </atom-group> -->
