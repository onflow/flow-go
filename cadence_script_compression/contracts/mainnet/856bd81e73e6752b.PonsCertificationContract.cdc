/*
	Pons Certification Contract

	This smart contract contains the PonsCertification resource, which acts as authorisation by the Pons account.
*/
pub contract PonsCertificationContract {

/*
	Pons Certification Resource

	This resource acts as authorisation by the Pons account.
*/
	pub resource PonsCertification {}

	/* Smart contracts on the Pons account can create PonsCertification instances using this function */
	access(account) fun makePonsCertification () : @PonsCertification {
		return <- create PonsCertification () } }
