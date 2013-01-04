/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2012 RELIC Authors
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file
 * for contact information.
 *
 * RELIC is free software; you can redistribute it and/or
 * modify it under the terms of the GNU Lesser General Public
 * License as published by the Free Software Foundation; either
 * version 2.1 of the License, or (at your option) any later version.
 *
 * RELIC is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with RELIC. If not, see <http://www.gnu.org/licenses/>.
 */

/**
 * @file
 *
 * Implementation of the prime elliptic curve utilities.
 *
 * @version $Id$
 * @ingroup ep
 */

#include "relic_core.h"
#include "relic_epx.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#if defined(EP_ORDIN) && FP_PRIME == 160
/**
 * Parameters for the SECG P-160 prime elliptic curve.
 */
/** @{ */
#define SECG_P160_A		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF7FFFFFFC"
#define SECG_P160_B		"1C97BEFC54BD7A8B65ACF89F81D4D4ADC565FA45"
#define SECG_P160_X		"4A96B5688EF573284664698968C38BB913CBFC82"
#define SECG_P160_Y		"23A628553168947D59DCC912042351377AC5FB32"
#define SECG_P160_R		"100000000000000000001F4C8F927AED3CA752257"
#define SECG_P160_H		"1"
/** @} */
#endif

#if defined(EP_KBLTZ) && FP_PRIME == 160
/**
 * Parameters for the SECG K-160 prime elliptic curve.
 */
/** @{ */
#define SECG_K160_A		"0"
#define SECG_K160_B		"7"
#define SECG_K160_X		"3B4C382CE37AA192A4019E763036F4F5DD4D7EBB"
#define SECG_K160_Y		"938CF935318FDCED6BC28286531733C3F03C4FEE"
#define SECG_K160_R		"100000000000000000001B8FA16DFAB9ACA16B6B3"
#define SECG_K160_H		"1"
#define SECG_K160_BETA	"645B7345A143464942CC46D7CF4D5D1E1E6CBB68"
#define SECG_K160_LAMB	"F3C6393C4C5C9288FE47F1DFF787A6EC6D16B2BE"
/** @} */
#endif

#if defined(EP_ORDIN) && FP_PRIME == 192
/**
 * Parameters for the NIST P-192 prime elliptic curve.
 */
/** @{ */
#define NIST_P192_A		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFFFFFFFFFFFC"
#define NIST_P192_B		"64210519E59C80E70FA7E9AB72243049FEB8DEECC146B9B1"
#define NIST_P192_X		"188DA80EB03090F67CBF20EB43A18800F4FF0AFD82FF1012"
#define NIST_P192_Y		"07192B95FFC8DA78631011ED6B24CDD573F977A11E794811"
#define NIST_P192_R		"FFFFFFFFFFFFFFFFFFFFFFFF99DEF836146BC9B1B4D22831"
#define NIST_P192_H		"1"
/** @} */
#endif

#if defined(EP_KBLTZ) && FP_PRIME == 192
/**
 * Parameters for the SECG K-192 prime elliptic curve.
 */
/** @{ */
#define SECG_K192_A		"0"
#define SECG_K192_B		"3"
#define SECG_K192_X		"DB4FF10EC057E9AE26B07D0280B7F4341DA5D1B1EAE06C7D"
#define SECG_K192_Y		"9B2F2F6D9C5628A7844163D015BE86344082AA88D95E2F9D"
#define SECG_K192_R		"FFFFFFFFFFFFFFFFFFFFFFFE26F2FC170F69466A74DEFD8D"
#define SECG_K192_H		"1"
#define SECG_K192_BETA	"447A96E6C647963E2F7809FEAAB46947F34B0AA3CA0BBA74"
#define SECG_K192_LAMB	"C27B0D93EDDC7284B0C2AE9813318686DBB7A0EA73692CDB"
/** @} */
#endif

#if defined(EP_ORDIN) && FP_PRIME == 224
/**
 * Parameters for the NIST P-192 prime elliptic curve.
 */
/** @{ */
#define NIST_P224_A		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFFFFFFFFFFFFFFFFFFFE"
#define NIST_P224_B		"B4050A850C04B3ABF54132565044B0B7D7BFD8BA270B39432355FFB4"
#define NIST_P224_X		"B70E0CBD6BB4BF7F321390B94A03C1D356C21122343280D6115C1D21"
#define NIST_P224_Y		"BD376388B5F723FB4C22DFE6CD4375A05A07476444D5819985007E34"
#define NIST_P224_R		"FFFFFFFFFFFFFFFFFFFFFFFFFFFF16A2E0B8F03E13DD29455C5C2A3D"
#define NIST_P224_H		"1"
/** @} */
#endif

#if defined(EP_KBLTZ) && FP_PRIME == 224
/**
 * Parameters for the SECG K-224 prime elliptic curve.
 */
/** @{ */
#define SECG_K224_A		"0"
#define SECG_K224_B		"5"
#define SECG_K224_X		"A1455B334DF099DF30FC28A169A467E9E47075A90F7E650EB6B7A45C"
#define SECG_K224_Y		"7E089FED7FBA344282CAFBD6F7E319F7C0B0BD59E2CA4BDB556D61A5"
#define SECG_K224_R		"10000000000000000000000000001DCE8D2EC6184CAF0A971769FB1F7"
#define SECG_K224_H		"1"
#define SECG_K224_BETA	"FE0E87005B4E83761908C5131D552A850B3F58B749C37CF5B84D6768"
#define SECG_K224_LAMB	"60DCD2104C4CBC0BE6EEEFC2BDD610739EC34E317F9B33046C9E4788"
/** @} */
#endif

#if defined(EP_ORDIN) && FP_PRIME == 256
/**
 * Parameters for the NIST P-256 prime elliptic curve.
 */
/** @{ */
#define NIST_P256_A		"FFFFFFFF00000001000000000000000000000000FFFFFFFFFFFFFFFFFFFFFFFC"
#define NIST_P256_B		"5AC635D8AA3A93E7B3EBBD55769886BC651D06B0CC53B0F63BCE3C3E27D2604B"
#define NIST_P256_X		"6B17D1F2E12C4247F8BCE6E563A440F277037D812DEB33A0F4A13945D898C296"
#define NIST_P256_Y		"4FE342E2FE1A7F9B8EE7EB4A7C0F9E162BCE33576B315ECECBB6406837BF51F5"
#define NIST_P256_R		"FFFFFFFF00000000FFFFFFFFFFFFFFFFBCE6FAADA7179E84F3B9CAC2FC632551"
#define NIST_P256_H		"1"
/** @} */
#endif

#if defined(EP_KBLTZ) && FP_PRIME == 256
/**
 * Parameters for the SECG K-256 prime elliptic curve.
 */
/** @{ */
#define SECG_K256_A		"0"
#define SECG_K256_B		"7"
#define SECG_K256_X		"79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798"
#define SECG_K256_Y		"483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8"
#define SECG_K256_R		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141"
#define SECG_K256_H		"1"
#define SECG_K256_BETA	"7AE96A2B657C07106E64479EAC3434E99CF0497512F58995C1396C28719501EE"
#define SECG_K256_LAMB	"5363AD4CC05C30E0A5261C028812645A122E22EA20816678DF02967C1B23BD72"
/** @} */
#endif

#if defined(EP_ORDIN) && FP_PRIME == 384
/**
 * Parameters for the NIST P-192 prime elliptic curve.
 */
/** @{ */
#define NIST_P384_A		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFFFF0000000000000000FFFFFFFC"
#define NIST_P384_B		"B3312FA7E23EE7E4988E056BE3F82D19181D9C6EFE8141120314088F5013875AC656398D8A2ED19D2A85C8EDD3EC2AEF"
#define NIST_P384_X		"AA87CA22BE8B05378EB1C71EF320AD746E1D3B628BA79B9859F741E082542A385502F25DBF55296C3A545E3872760AB7"
#define NIST_P384_Y		"3617DE4A96262C6F5D9E98BF9292DC29F8F41DBD289A147CE9DA3113B5F0B8C00A60B1CE1D7E819D7A431D7C90EA0E5F"
#define NIST_P384_R		"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFC7634D81F4372DDF581A0DB248B0A77AECEC196ACCC52973"
#define NIST_P384_H		"1"
/** @} */
#endif

#if defined(EP_ORDIN) && FP_PRIME == 521
/**
 * Parameters for the NIST P-192 prime elliptic curve.
 */
/** @{ */
#define NIST_P521_A		"1FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFC"
#define NIST_P521_B		"51953EB9618E1C9A1F929A21A0B68540EEA2DA725B99B315F3B8B489918EF109E156193951EC7E937B1652C0BD3BB1BF073573DF883D2C34F1EF451FD46B503F00"
#define NIST_P521_X		"C6858E06B70404E9CD9E3ECB662395B4429C648139053FB521F828AF606B4D3DBAA14B5E77EFE75928FE1DC127A2FFA8DE3348B3C1856A429BF97E7E31C2E5BD66"
#define NIST_P521_Y		"11839296A789A3BC0045C8A5FB42C7D1BD998F54449579B446817AFBD17273E662C97EE72995EF42640C550B9013FAD0761353C7086A272C24088BE94769FD16650"
#define NIST_P521_R		"1FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFA51868783BF2F966B7FCC0148F709A5D03BB5C9B8899C47AEBB6FB71E91386409"
#define NIST_P521_H		"1"
/** @} */
#endif

#if defined(EP_KBLTZ) && FP_PRIME == 158
/**
 * Parameters for a 158-bit pairing-friendly prime curve.
 */
/** @{ */
#define BN_P158_A		"0"
#define BN_P158_B		"11"
#define BN_P158_X		"240000006ED000007FE9C000419FEC800CA035C6"
#define BN_P158_Y		"4"
#define BN_P158_R		"240000006ED000007FE96000419F59800C9FFD81"
#define BN_P158_H		"1"
#define BN_P158_BETA	"48000000A68000008058C00020FABE"
#define BN_P158_LAMB	"900000014BE00000FEF58000414A5D"
/** @} */
#endif

#if defined(EP_KBLTZ) && FP_PRIME == 254
/**
 * Parameters for a 254-bit pairing-friendly prime curve.
 */
/** @{ */
#define BN_P254_A		"0"
#define BN_P254_B		"2"
#define BN_P254_X		"2523648240000001BA344D80000000086121000000000013A700000000000012"
#define BN_P254_Y		"1"
#define BN_P254_R		"2523648240000001BA344D8000000007FF9F800000000010A10000000000000D"
#define BN_P254_H		"1"
#define BN_P254_BETA	"25236482400000017080EB4000000006181800000000000CD98000000000000B"
#define BN_P254_LAMB	"252364824000000126CD8900000000024908FFFFFFFFFFFCF9FFFFFFFFFFFFF6"
/** @} */
#endif

#if defined(EP_KBLTZ) && FP_PRIME == 256
/**
 * Parameters for a 256-bit pairing-friendly prime curve.
 */
/** @{ */
#define BN_P256_A		"0"
#define BN_P256_B		"11"
#define BN_P256_X		"B64000000000FF2F2200000085FD5480B0001F44B6B88BF142BC818F95E3E6AE"
#define BN_P256_Y		"4"
#define BN_P256_R		"B64000000000FF2F2200000085FD547FD8001F44B6B7F4B7C2BC818F7B6BEF99"
#define BN_P256_H		"1"
#define BN_P256_BETA	"B64000000000FF2E2F00000085FC555230001F445D656FB022BC77236CD54C89"
#define BN_P256_LAMB	"B64000000000FF2D3C00000085FB562050001F44040FF68D82BC6CB6D9E8694E"
/** @} */
#endif

#if defined(EP_KBLTZ) && FP_PRIME == 477
/**
 * Parameters for a 477-bit pairing-friendly prime curve at the 192-bit security level.
 */
/** @{ */
#define B24_P477_A		"0"
#define B24_P477_B		"4"
#define B24_P477_X		"15DFD8E4893A408A34B600532B51CC86CAB3AF07103CFCF3EC7B9AF836904CFB60AB0FA8AC91EE6255E5EF6286FA0C24DF9D76EA50599C2E103E40AD"
#define B24_P477_Y		"0A683957A59B1B488FA657E11B44815056BDE33C09D6AAD392D299F89C7841B91A683BF01B7E70547E48E0FBE1CA9E991983131470F886BA9B6FCE2E"
#define B24_P477_R		"57F52EE445CC41781FCD53D13E45F6ACDFE4F9F2A3CD414E71238AFC9FCFC7D38CAEF64F4FF79F90013FFFFFF0000001"
#define B24_P477_H		"41550AAAC04B3FD5000015AB"
#define B24_P477_BETA	"926C960A5EC3B3A6C6B9CEF2CB923D3240E4780BC1AE423EE39586AD923B1C949768022369DD2CE502E7FCA0670B3A996AC44B48B523DAA7390CCB1F6D9012F"
#define B24_P477_LAMB	"1001740B431D14BFD17F4BD000300173FFFFFFFEFFFFFFFED"
/** @} */
#endif

#if defined(EP_KBLTZ) && FP_PRIME == 508
/**
 * Parameters for a 508-bit pairing-friendly prime curve at the 192-bit security level.
 */
/** @{ */
#define KSS_P508_A		"0"
#define KSS_P508_B		"2"
#define KSS_P508_X		"3ADD59EAC7B6A0ABC781139CE46388AB3426C6619C27187CB1F2B48AC92E04608AFBD25DA121EDB06015CB5D2BCF369C03C163605BBA21FAF7D550960553784"
#define KSS_P508_Y		"8773227730CBE52483BF6AAAA9E4FE2870B463FA14D92C31D0F99C6B6EE13106A0E8C87AD7631F8ECCE0DD6189B4C2232C644E4B857F325923FC8A80A947FFA"
#define KSS_P508_R		"BF33E1C9934E7868ECE51D291E5644DA8A2F179CEE74854EE6819B240F20CE4E7D19F4CDABA6EAEA5B0E3000000001"
#define KSS_P508_H		"10565283D505534A492ADC6AAABB051B1D"
#define KSS_P508_BETA	"926C960A5EC3B3A6C6B9CEF2CB923D3240E4780BC1AE423EE39586AD923B1C949768022369DD2CE502E7FCA0670B3A996AC44B48B523DAA7390CCB1F6D9012F"
#define KSS_P508_LAMB	"1001740B431D14BFD17F4BD000300173FFFFFFFEFFFFFFFED"
/** @} */
#endif

#if defined(EP_KBLTZ) && FP_PRIME == 638
/**
 * Parameters for a 638-bit pairing-friendly prime curve.
 */
/** @{ */
#define BN_P638_A		"0"
#define BN_P638_B		"5"
#define BN_P638_X		"17F382B46725AEFA6F38E6F87ACAA1ACA41757FC6F2C40DB6539EDEB8B95D519B1E99255DF031C67DE6C00E39547E6347E80E5F3D36CC5B4B42FC10D061BFA0A0A8C6CB6B561C487A1420E7B383DC742"
#define BN_P638_Y		"128AC488584B7C05EFD5436E559D741C978A5027926525B3DECB22D40E03FC7BD8D4235FD7E9DD2F3BFF3945D54C25E701624E27AFEF8F27F7DDEADEDAF3FE3AA0234D35290703FCE6254A7D75B6A304"
#define BN_P638_R		"23FFFFFDC000000D7FFFFFB8000001D3FFFFF942D000165E3FFF94870000D52FFFFDD0E00008DE55600086550021E555FFFFF54FFFF4EAC000000049800154D9FFFFFFFFFFFFEDA00000000000000061"
#define BN_P638_H		"1"
#define BN_P638_BETA	"47FFFFFCA000000D7FFFFFB8000001AFFFFFFCA480000D5BFFFFCA47FFFFFDBFFFFEE90000000018C000479CFFFFFFFFFFFFF9D0000000000000002E"
#define BN_P638_LAMB	"8FFFFFF94000001AFFFFFF700000035FFFFFF947E0001AC0FFFF947DFFFFFC0FFFFDCFC00000002580007D69FFFFFFFFFFFFF6A0000000000000003D"
/** @} */
#endif

#if defined(EP_KBLTZ) && FP_PRIME == 638
/**
 * Parameters for a 638-bit pairing-friendly prime curve.
 */
/** @{ */
#define B12_P638_A		"0"
#define B12_P638_B		"4"
#define B12_P638_X		"160F63A3A3B297F113075ED79466138E85B025F7FE724B78E32D7AFC4D734BDD54F871092B8D1966D491C0F45A48A8BBA5586095DFFCC1410B7E26ED16BAF98C1117959134C24A17A7BE31E1AFBF844F"
#define B12_P638_Y		"2D340B33877480A9785E86ED2EDCAFC170B82568CB21B708B79FC6DA3748461FCD80697E486695F3CAE76FCB1781E784F6812F57BE05DFC850426650DED8B40A464B00A35718228EC8E02B52B59D876E"
#define B12_P638_R		"50F94035FF4000FFFFFFFFFFF9406BFDC0040000000000000035FB801DFFBFFFFFFFFFFFFFFF401BFF80000000000000000000FFC01"
#define B12_P638_H		"1"
#define B12_P638_BETA	"1E5CD621BF4C01DFFDFFFFFFFCE57239EE275BF63000000000207C802254C4BE7FFFFFFFFFFF55FE959D1B8000000000000001BCCEB5801FFFFFFFFFFFFFFFFE2E801E"
#define B12_P638_LAMB	"50F94035FF4000FFFFFFFFFFF9406BFDC0040000000000000035F94035FF7FFFFFFFFFFFFFFF4033FF00000000000000000000FF801"
/** @} */
#endif

/**
 * Assigns a set of ordinary elliptic curve parameters.
 *
 * @param[in] CURVE		- the curve parameters to assign.
 * @param[in] FIELD		- the finite field identifier.
 */
#define ASSIGN(CURVE, FIELD)												\
	fp_param_set(FIELD);													\
	FETCH(str, CURVE##_A, sizeof(CURVE##_A));								\
	fp_read(a, str, strlen(str), 16);										\
	FETCH(str, CURVE##_B, sizeof(CURVE##_B));								\
	fp_read(b, str, strlen(str), 16);										\
	FETCH(str, CURVE##_X, sizeof(CURVE##_X));								\
	fp_read(g->x, str, strlen(str), 16);									\
	FETCH(str, CURVE##_Y, sizeof(CURVE##_Y));								\
	fp_read(g->y, str, strlen(str), 16);									\
	FETCH(str, CURVE##_R, sizeof(CURVE##_R));								\
	bn_read_str(r, str, strlen(str), 16);									\
	FETCH(str, CURVE##_H, sizeof(CURVE##_H));								\
	bn_read_str(h, str, strlen(str), 16);									\

/**
 * Assigns a set of Koblitz elliptic curve parameters.
 *
 * @param[in] CURVE		- the curve parameters to assign.
 * @param[in] FIELD		- the finite field identifier.
 */
#define ASSIGNK(CURVE, FIELD)												\
	ASSIGN(CURVE, FIELD);													\
	FETCH(str, CURVE##_BETA, sizeof(CURVE##_BETA));							\
	fp_read(beta, str, strlen(str), 16);									\
	FETCH(str, CURVE##_LAMB, sizeof(CURVE##_LAMB));							\
	bn_read_str(lamb, str, strlen(str), 16);								\

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

int ep_param_get() {
	return core_get()->ep_id;
}

void ep_param_set(int param) {
	int ordin = 0;
	int kbltz = 0;
	char str[2 * FP_BYTES + 1];
	fp_t a, b, beta;
	ep_t g;
	bn_t r, h, lamb;

	fp_null(a);
	fp_null(b);
	fp_null(beta);
	bn_null(lamb);
	ep_null(g);
	bn_null(r);
	bn_null(h);

	TRY {
		fp_new(a);
		fp_new(b);
		fp_new(beta);
		bn_new(lamb);
		ep_new(g);
		bn_new(r);
		bn_new(h);

		switch (param) {
#if defined(EP_KBLTZ) && FP_PRIME == 158
			case BN_P158:
				ASSIGNK(BN_P158, BN_158);
				kbltz = 1;
				break;
#endif
#if defined(EP_ORDIN) && FP_PRIME == 160
			case SECG_P160:
				ASSIGN(SECG_P160, SECG_160);
				ordin = 1;
				break;
#endif
#if defined(EP_KBLTZ) && FP_PRIME == 160
			case SECG_K160:
				ASSIGNK(SECG_K160, SECG_160D);
				kbltz = 1;
				break;
#endif
#if defined(EP_ORDIN) && FP_PRIME == 192
			case NIST_P192:
				ASSIGN(NIST_P192, NIST_192);
				ordin = 1;
				break;
#endif
#if defined(EP_KBLTZ) && FP_PRIME == 192
			case SECG_K192:
				ASSIGNK(SECG_K192, SECG_192);
				kbltz = 1;
				break;
#endif
#if defined(EP_ORDIN) && FP_PRIME == 224
			case NIST_P224:
				ASSIGN(NIST_P224, NIST_224);
				ordin = 1;
				break;
#endif
#if defined(EP_KBLTZ) && FP_PRIME == 224
			case SECG_K224:
				ASSIGNK(SECG_K224, SECG_224);
				kbltz = 1;
				break;
#endif
#if defined(EP_KBLTZ) && FP_PRIME == 254
			case BN_P254:
				ASSIGNK(BN_P254, BN_254);
				kbltz = 1;
				break;
#endif
#if defined(EP_ORDIN) && FP_PRIME == 256
			case NIST_P256:
				ASSIGN(NIST_P256, NIST_256);
				ordin = 1;
				break;
#endif
#if defined(EP_KBLTZ) && FP_PRIME == 256
			case SECG_K256:
				ASSIGNK(SECG_K256, SECG_256);
				kbltz = 1;
				break;
			case BN_P256:
				ASSIGNK(BN_P256, BN_256);
				kbltz = 1;
				break;
#endif
#if defined(EP_ORDIN) && FP_PRIME == 384
			case NIST_P384:
				ASSIGN(NIST_P384, NIST_384);
				ordin = 1;
				break;
#endif
#if defined(EP_ORDIN) && FP_PRIME == 477
			case B24_P477:
				ASSIGN(B24_P477, B24_477);
				ordin = 1;
				break;
#endif
#if defined(EP_ORDIN) && FP_PRIME == 508
			case KSS_P508:
				ASSIGNK(KSS_P508, KSS_508);
				kbltz = 1;
				break;
#endif
#if defined(EP_ORDIN) && FP_PRIME == 521
			case NIST_P521:
				ASSIGN(NIST_P521, NIST_521);
				ordin = 1;
				break;
#endif
#if defined(EP_KBLTZ) && FP_PRIME == 638
			case BN_P638:
				ASSIGNK(BN_P638, BN_638);
				kbltz = 1;
				break;
			case B12_P638:
				ASSIGNK(B12_P638, B12_638);
				kbltz = 1;
				break;
#endif
			default:
				(void)str;
				THROW(ERR_NO_VALID);
				break;
		}

		/* Do not generate warnings. */
		(void)kbltz;
		(void)ordin;
		(void)beta;

		core_get()->ep_id = param;

		fp_zero(g->z);
		fp_set_dig(g->z, 1);
		g->norm = 1;

#if defined(EP_ORDIN)
		if (ordin) {
			ep_curve_set_ordin(a, b, g, r, h);
		}
#endif

#if defined(EP_KBLTZ)
		if (kbltz) {
			ep_curve_set_kbltz(b, g, r, h, beta, lamb);
		}
#endif
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp_free(a);
		fp_free(b);
		fp_free(beta);
		bn_free(lamb);
		ep_free(g);
		bn_free(r);
		bn_free(h);
	}
}

int ep_param_set_any() {
	int r0, r1;

	r0 = ep_param_set_any_ordin();
	if (r0 == STS_ERR) {
		r1 = ep_param_set_any_pairf();
		if (r1 == STS_ERR) {
			return STS_ERR;
		}
	}
	return STS_OK;
}

int ep_param_set_any_ordin() {
	int r = STS_OK;
#if FP_PRIME == 160
	ep_param_set(SECG_P160);
#elif FP_PRIME == 192
	ep_param_set(NIST_P192);
#elif FP_PRIME == 224
	ep_param_set(NIST_P224);
#elif FP_PRIME == 256
	ep_param_set(NIST_P256);
#elif FP_PRIME == 384
	ep_param_set(NIST_P384);
#elif FP_PRIME == 521
	ep_param_set(NIST_P521);
#else
	r = STS_ERR;
#endif
	return r;
}

int ep_param_set_any_kbltz() {
	int r = STS_OK;
#if FP_PRIME == 158
	ep_param_set(BN_P158);
#elif FP_PRIME == 160
	ep_param_set(SECG_K160);
#elif FP_PRIME == 192
	ep_param_set(SECG_K192);
#elif FP_PRIME == 224
	ep_param_set(SECG_K224);
#elif FP_PRIME == 254
	ep_param_set(BN_P254);
#elif FP_PRIME == 256
	ep_param_set(SECG_K256);
#elif FP_PRIME == 477
	ep_param_set(B24_P477);
#elif FP_PRIME == 508
	ep_param_set(KSS_P508);
#elif FP_PRIME == 638
	ep_param_set(BN_P638);
#else
	r = STS_ERR;
#endif
	return r;
}

int ep_param_set_any_pairf() {
	int twist, degree, r = STS_OK;
#if FP_PRIME == 158
	ep_param_set(BN_P158);
	twist = EP_DTYPE;
	degree = 2;
#elif FP_PRIME == 254
	ep_param_set(BN_P254);
	twist = EP_DTYPE;
	degree = 2;
#elif FP_PRIME == 256
	ep_param_set(BN_P256);
	twist = EP_DTYPE;
	degree = 2;
#elif FP_PRIME == 477
	ep_param_set(B24_P477);
	twist = EP_MTYPE;
	degree = 4;
#elif FP_PRIME == 508
	ep_param_set(KSS_P508);
	twist = EP_DTYPE;
	degree = 3;
#elif FP_PRIME == 638
	ep_param_set(B12_P638);
	twist = EP_MTYPE;
	degree = 2;
#else
	r = STS_ERR;
#endif
#ifdef WITH_PP
	if (r == STS_OK) {
		if (degree == 2) {
			ep2_curve_set(twist);
		}
		if (degree == 3 || degree == 4) {
			r = STS_ERR;
		}
	}
#else
	(void)twist;
	(void)degree;
#endif
	return r;
}

void ep_param_print() {
	switch (ep_param_get()) {
		case SECG_P160:
			util_banner("Curve SECG-P160:", 0);
			break;
		case SECG_K160:
			util_banner("Curve SECG-K160:", 0);
			break;
		case NIST_P192:
			util_banner("Curve NIST-P192:", 0);
			break;
		case SECG_K192:
			util_banner("Curve SECG-K192:", 0);
			break;
		case NIST_P224:
			util_banner("Curve NIST-P224:", 0);
			break;
		case SECG_K224:
			util_banner("Curve SECG-K224:", 0);
			break;
		case NIST_P256:
			util_banner("Curve NIST-P256:", 0);
			break;
		case SECG_K256:
			util_banner("Curve SECG-K256:", 0);
			break;
		case NIST_P384:
			util_banner("Curve NIST-P384:", 0);
			break;
		case NIST_P521:
			util_banner("Curve NIST-P521:", 0);
			break;
		case BN_P158:
			util_banner("Curve BN-P158:", 0);
			break;
		case BN_P254:
			util_banner("Curve BN-P254:", 0);
			break;
		case BN_P256:
			util_banner("Curve BN-P256:", 0);
			break;
		case B24_P477:
			util_banner("Curve B24-P477:", 0);
			break;
		case KSS_P508:
			util_banner("Curve KSS-P508:", 0);
			break;
		case BN_P638:
			util_banner("Curve BN-P638:", 0);
			break;
		case B12_P638:
			util_banner("Curve B12-P638:", 0);
			break;
	}
}

int ep_param_level() {
	switch (ep_param_get()) {
		case BN_P158:
			return 78;
		case SECG_P160:
		case SECG_K160:
			return 80;
		case NIST_P192:
		case SECG_K192:
			return 96;
		case NIST_P224:
		case SECG_K224:
			return 112;
		case BN_P254:
			return 126;
		case NIST_P256:
		case SECG_K256:
		case BN_P256:
			return 128;
		case NIST_P384:
			return 192;
		case NIST_P521:
			return 256;
		case BN_P638:
		case B12_P638:
			return 192;
	}
	return 0;
}
