/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2013 RELIC Authors
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
 * Symbol renaming to avoid clashes when simultaneous linking multiple builds.
 *
 * @version $Id$
 * @ingroup core
 */

#ifndef RELIC_LABEL_H
#define RELIC_LABEL_H

#include "relic_conf.h"

#define PREFIX(F)			_PREFIX(LABEL, F)
#define _PREFIX(A, B)		__PREFIX(A, B)
#define __PREFIX(A, B)		A ## B

/*============================================================================*/
/* Macro definitions                                                          */
/*============================================================================*/

#ifdef LABEL

#undef core_init
#undef core_clean
#undef core_get
#undef core_set
#undef core_ctx

#define core_init			PREFIX(_core_init)
#define core_clean			PREFIX(_core_clean)
#define core_set			PREFIX(_core_set)
#define core_get			PREFIX(_core_get)
#define	core_ctx			PREFIX(_core_ctx)

#undef rand_init
#undef rand_clean
#undef rand_seed
#undef rand_bytes

#define rand_init			PREFIX(_rand_init)
#define rand_clean			PREFIX(_rand_clean)
#define rand_seed			PREFIX(_rand_seed)
#define rand_bytes			PREFIX(_rand_bytes)

#undef arch_init
#undef arch_clean
#undef arch_cycles
#undef arch_copy_rom

#define arch_init			PREFIX(_arch_init)
#define arch_clean			PREFIX(_arch_clean)
#define arch_cycles			PREFIX(_arch_cycles)
#define arch_copy_rom		PREFIX(_arch_copy_rom)

#undef bn_st
#undef bn_t
#undef bn_init
#undef bn_clean
#undef bn_grow
#undef bn_trim
#undef bn_copy
#undef bn_abs
#undef bn_neg
#undef bn_sign
#undef bn_zero
#undef bn_is_zero
#undef bn_is_even
#undef bn_bits
#undef bn_test_bit
#undef bn_get_bit
#undef bn_set_bit
#undef bn_ham
#undef bn_get_dig
#undef bn_set_dig
#undef bn_rand
#undef bn_print
#undef bn_size_str
#undef bn_read_str
#undef bn_write_str
#undef bn_size_bin
#undef bn_read_bin
#undef bn_write_bin
#undef bn_size_raw
#undef bn_read_raw
#undef bn_write_raw
#undef bn_cmp_abs
#undef bn_cmp_dig
#undef bn_cmp
#undef bn_add
#undef bn_add_dig
#undef bn_sub
#undef bn_sub_sig
#undef bn_mul_dig
#undef bn_mul_basic
#undef bn_mul_comba
#undef bn_mul_karat
#undef bn_sqr_basic
#undef bn_sqr_comba
#undef bn_sqr_karat
#undef bn_dbl
#undef bn_hlv
#undef bn_lsh
#undef bn_rsh
#undef bn_div
#undef bn_div_rem
#undef bn_div_dig
#undef bn_div_rem_dig
#undef bn_mod_2b
#undef bn_mod_dig
#undef bn_mod_basic
#undef bn_mod_pre_barrt
#undef bn_mod_barrt
#undef bn_mod_pre_monty
#undef bn_mod_monty_conv
#undef bn_mod_monty_back
#undef bn_mod_monty_basic
#undef bn_mod_monty_comba
#undef bn_mod_pre_pmers
#undef bn_mod_pmers
#undef bn_mxp_basic
#undef bn_mxp_slide
#undef bn_mxp_monty
#undef bn_mxp_dig
#undef bn_gcd_basic
#undef bn_gcd_lehme
#undef bn_gcd_stein
#undef bn_gcd_dig
#undef bn_gcd_ext_basic
#undef bn_gcd_ext_lehme
#undef bn_gcd_ext_stein
#undef bn_gcd_ext_mid
#undef bn_gcd_ext_dig
#undef bn_lcm
#undef bn_smb_leg
#undef bn_smb_jac
#undef bn_is_prime
#undef bn_is_prime_basic
#undef bn_is_prime_rabin
#undef bn_is_prime_solov
#undef bn_gen_prime_basic
#undef bn_gen_prime_safep
#undef bn_gen_prime_stron
#undef bn_factor
#undef bn_is_factor
#undef bn_rec_win
#undef bn_rec_slw
#undef bn_rec_naf
#undef bn_rec_tnaf
#undef bn_rec_jsf
#undef bn_rec_glv

#define bn_st				PREFIX(_bn_st)
#define bn_t				PREFIX(_bn_t)
#define bn_init				PREFIX(_bn_init)
#define bn_clean			PREFIX(_bn_clean)
#define bn_grow				PREFIX(_bn_grow)
#define bn_trim				PREFIX(_bn_trim)
#define bn_copy				PREFIX(_bn_copy)
#define bn_abs				PREFIX(_bn_abs)
#define bn_neg				PREFIX(_bn_neg)
#define bn_sign				PREFIX(_bn_sign)
#define bn_zero				PREFIX(_bn_zero)
#define bn_is_zero			PREFIX(_bn_is_zero)
#define bn_is_even			PREFIX(_bn_is_even)
#define bn_test_bit			PREFIX(_bn_test_bit)
#define bn_get_bit			PREFIX(_bn_get_bit)
#define bn_set_bit			PREFIX(_bn_set_bit)
#define bn_ham				PREFIX(_bn_ham)
#define bn_get_dig			PREFIX(_bn_get_dig)
#define bn_set_dig			PREFIX(_bn_set_dig)
#define bn_set_2b			PREFIX(_bn_set_2b)
#define bn_rand				PREFIX(_bn_rand)
#define bn_print			PREFIX(_bn_print)
#define bn_size_str			PREFIX(_bn_size_str)
#define bn_read_str			PREFIX(_bn_read_str)
#define bn_write_str		PREFIX(_bn_write_str)
#define bn_size_bin			PREFIX(_bn_size_bin)
#define bn_read_bin			PREFIX(_bn_read_bin)
#define bn_write_bin		PREFIX(_bn_write_bin)
#define bn_size_raw			PREFIX(_bn_size_raw)
#define bn_read_raw			PREFIX(_bn_read_raw)
#define bn_write_raw		PREFIX(_bn_write_raw)
#define bn_cmp_abs			PREFIX(_bn_cmp_abs)
#define bn_cmp_dig			PREFIX(_bn_cmp_dig)
#define bn_cmp				PREFIX(_bn_cmp)
#define bn_add				PREFIX(_bn_add)
#define bn_add_dig			PREFIX(_bn_add_dig)
#define bn_sub				PREFIX(_bn_sub)
#define bn_sub_sig			PREFIX(_bn_sub_dig)
#define bn_mul_dig			PREFIX(_bn_mul_dig)
#define bn_mul_basic		PREFIX(_bn_mul_basic)
#define bn_mul_comba		PREFIX(_bn_mul_comba)
#define bn_mul_karat		PREFIX(_bn_mul_karat)
#define bn_sqr_basic		PREFIX(_bn_sqr_basic)
#define bn_sqr_comba		PREFIX(_bn_sqr_comba)
#define bn_sqr_karat		PREFIX(_bn_sqr_karat)
#define bn_dbl				PREFIX(_bn_dbl)
#define bn_hlv				PREFIX(_bn_hlv)
#define bn_lsh				PREFIX(_bn_lsh)
#define bn_rsh				PREFIX(_bn_rsh)
#define bn_div				PREFIX(_bn_div)
#define bn_div_rem			PREFIX(_bn_div_rem)
#define bn_div_dig			PREFIX(_bn_div_dig)
#define bn_div_rem_dig		PREFIX(_bn_div_rem_dig)
#define bn_mod_2b			PREFIX(_bn_mod_2b)
#define bn_mod_dig			PREFIX(_bn_mod_dig)
#define bn_mod_basic		PREFIX(_bn_mod_basic)
#define bn_mod_pre_barrt	PREFIX(_bn_mod_pre_barrt)
#define bn_mod_barrt		PREFIX(_bn_mod_barrt)
#define bn_mod_pre_monty	PREFIX(_bn_mod_pre_monty)
#define bn_mod_monty_conv	PREFIX(_bn_mod_monty_conv)
#define bn_mod_monty_back	PREFIX(_bn_mod_monty_back)
#define bn_mod_monty_basic	PREFIX(_bn_mod_monty_basic)
#define bn_mod_monty_comba	PREFIX(_bn_mod_monty_comba)
#define bn_mod_pre_pmers	PREFIX(_bn_mod_pre_pmers)
#define bn_mod_pmers		PREFIX(_bn_mod_perms)
#define bn_mxp_basic		PREFIX(_bn_mxp_basic)
#define bn_mxp_slide		PREFIX(_bn_mxp_slide)
#define bn_mxp_monty		PREFIX(_bn_mxp_monty)
#define bn_mxp_dig			PREFIX(_bn_mxp_dig)
#define bn_gcd_basic		PREFIX(_bn_gcd_basic)
#define bn_gcd_lehme		PREFIX(_bn_gcd_lehme)
#define bn_gcd_stein		PREFIX(_bn_gcd_stein)
#define bn_gcd_dig			PREFIX(_bn_gcd_dig)
#define bn_gcd_ext_basic	PREFIX(_bn_gcd_ext_basic)
#define bn_gcd_ext_lehme	PREFIX(_bn_gcd_ext_lehme)
#define bn_gcd_ext_stein	PREFIX(_bn_gcd_ext_stein)
#define bn_gcd_ext_mid		PREFIX(_bn_gcd_ext_mid)
#define bn_gcd_ext_dig		PREFIX(_bn_gcd_ext_dig)
#define bn_lcm				PREFIX(_bn_lcm)
#define bn_smb_leg			PREFIX(_bn_smb_leg)
#define bn_smb_jac			PREFIX(_bn_smb_jac)
#define bn_is_prime			PREFIX(_bn_is_prime)
#define bn_is_prime_basic	PREFIX(_bn_is_prime_basic)
#define bn_is_prime_rabin	PREFIX(_bn_is_prime_rabin)
#define bn_is_prime_solov	PREFIX(_bn_is_prime_solov)
#define bn_gen_prime_basic	PREFIX(_bn_gen_prime_basic)
#define bn_gen_prime_safep	PREFIX(_bn_gen_prime_safep)
#define bn_gen_prime_stron	PREFIX(_bn_gen_prime_stron)
#define bn_factor			PREFIX(_bn_factor)
#define bn_is_factor		PREFIX(_bn_is_factor)
#define bn_rec_win			PREFIX(_bn_rec_win)
#define bn_rec_slw			PREFIX(_bn_rec_slw)
#define bn_rec_naf			PREFIX(_bn_rec_naf)
#define bn_rec_tnaf			PREFIX(_bn_rec_tnaf)
#define bn_rec_jsf			PREFIX(_bn_rec_jsf)
#define bn_rec_glv			PREFIX(_bn_rec_glv)

#endif /* LABEL */

#endif /* !RELIC_LABEL_H */
