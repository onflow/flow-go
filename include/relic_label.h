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
 * Symbol renaming to a#undef clashes when simultaneous linking multiple builds.
 *
 * @version $Id$
 * @ingroup core
 */

#ifndef RELIC_LABEL_H
#define RELIC_LABEL_H

#include "relic_conf.h"

#define PREFIX(F)			_PREFIX(LABEL, F)
#define _PREFIX(A, B)		__PREFIX(A, B)
#define __PREFIX(A, B)		A ## _ ## B

/*============================================================================*/
/* Macro definitions                                                          */
/*============================================================================*/

#ifdef LABEL

#undef core_init
#undef core_clean
#undef core_get
#undef core_set

#define core_init 	PREFIX(core_init)
#define core_clean 	PREFIX(core_clean)
#define core_get 	PREFIX(core_get)
#define core_set 	PREFIX(core_set)

#undef arch_init
#undef arch_clean
#undef arch_cycles
#undef arch_copy_rom

#define arch_init 	PREFIX(arch_init)
#define arch_clean 	PREFIX(arch_clean)
#define arch_cycles 	PREFIX(arch_cycles)
#define arch_copy_rom 	PREFIX(arch_copy_rom)

#undef rand_init
#undef rand_clean
#undef rand_seed
#undef rand_bytes

#define rand_init 	PREFIX(rand_init)
#define rand_clean 	PREFIX(rand_clean)
#define rand_seed 	PREFIX(rand_seed)
#define rand_bytes 	PREFIX(rand_bytes)

#undef pool_get
#undef pool_put

#define pool_get 	PREFIX(pool_get)
#define pool_put 	PREFIX(pool_put)

#undef dv_t
#define dv_t	PREFIX(dv_t)

#undef dv_print
#undef dv_zero
#undef dv_copy
#undef dv_copy_cond
#undef dv_cmp_const
#undef dv_new_dynam
#undef dv_new_statc
#undef dv_free_dynam
#undef dv_free_statc

#define dv_print 	PREFIX(dv_print)
#define dv_zero 	PREFIX(dv_zero)
#define dv_copy 	PREFIX(dv_copy)
#define dv_copy_cond 	PREFIX(dv_copy_cond)
#define dv_cmp_const 	PREFIX(dv_cmp_const)
#define dv_new_dynam 	PREFIX(dv_new_dynam)
#define dv_new_statc 	PREFIX(dv_new_statc)
#define dv_free_dynam 	PREFIX(dv_free_dynam)
#define dv_free_statc 	PREFIX(dv_free_statc)

#undef bn_st
#undef bn_t
#define bn_st	PREFIX(bn_st)
#define bn_t	PREFIX(bn_t)

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
#undef bn_set_2b
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
#undef bn_sub_dig
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
#undef bn_get_prime
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
#undef bn_rec_reg
#undef bn_rec_jsf
#undef bn_rec_glv
#undef bn_add1_low
#undef bn_addn_low
#undef bn_sub1_low
#undef bn_subn_low
#undef bn_cmp1_low
#undef bn_cmpn_low
#undef bn_lsh1_low
#undef bn_lshb_low
#undef bn_lshd_low
#undef bn_rsh1_low
#undef bn_rshb_low
#undef bn_rshd_low
#undef bn_mula_low
#undef bn_mul1_low
#undef bn_muln_low
#undef bn_muld_low
#undef bn_sqra_low
#undef bn_sqrn_low
#undef bn_divn_low
#undef bn_div1_low
#undef bn_modn_low

#define bn_init 	PREFIX(bn_init)
#define bn_clean 	PREFIX(bn_clean)
#define bn_grow 	PREFIX(bn_grow)
#define bn_trim 	PREFIX(bn_trim)
#define bn_copy 	PREFIX(bn_copy)
#define bn_abs 	PREFIX(bn_abs)
#define bn_neg 	PREFIX(bn_neg)
#define bn_sign 	PREFIX(bn_sign)
#define bn_zero 	PREFIX(bn_zero)
#define bn_is_zero 	PREFIX(bn_is_zero)
#define bn_is_even 	PREFIX(bn_is_even)
#define bn_bits 	PREFIX(bn_bits)
#define bn_test_bit 	PREFIX(bn_test_bit)
#define bn_get_bit 	PREFIX(bn_get_bit)
#define bn_set_bit 	PREFIX(bn_set_bit)
#define bn_ham 	PREFIX(bn_ham)
#define bn_get_dig 	PREFIX(bn_get_dig)
#define bn_set_dig 	PREFIX(bn_set_dig)
#define bn_set_2b 	PREFIX(bn_set_2b)
#define bn_rand 	PREFIX(bn_rand)
#define bn_print 	PREFIX(bn_print)
#define bn_size_str 	PREFIX(bn_size_str)
#define bn_read_str 	PREFIX(bn_read_str)
#define bn_write_str 	PREFIX(bn_write_str)
#define bn_size_bin 	PREFIX(bn_size_bin)
#define bn_read_bin 	PREFIX(bn_read_bin)
#define bn_write_bin 	PREFIX(bn_write_bin)
#define bn_size_raw 	PREFIX(bn_size_raw)
#define bn_read_raw 	PREFIX(bn_read_raw)
#define bn_write_raw 	PREFIX(bn_write_raw)
#define bn_cmp_abs 	PREFIX(bn_cmp_abs)
#define bn_cmp_dig 	PREFIX(bn_cmp_dig)
#define bn_cmp 	PREFIX(bn_cmp)
#define bn_add 	PREFIX(bn_add)
#define bn_add_dig 	PREFIX(bn_add_dig)
#define bn_sub 	PREFIX(bn_sub)
#define bn_sub_dig 	PREFIX(bn_sub_dig)
#define bn_mul_dig 	PREFIX(bn_mul_dig)
#define bn_mul_basic 	PREFIX(bn_mul_basic)
#define bn_mul_comba 	PREFIX(bn_mul_comba)
#define bn_mul_karat 	PREFIX(bn_mul_karat)
#define bn_sqr_basic 	PREFIX(bn_sqr_basic)
#define bn_sqr_comba 	PREFIX(bn_sqr_comba)
#define bn_sqr_karat 	PREFIX(bn_sqr_karat)
#define bn_dbl 	PREFIX(bn_dbl)
#define bn_hlv 	PREFIX(bn_hlv)
#define bn_lsh 	PREFIX(bn_lsh)
#define bn_rsh 	PREFIX(bn_rsh)
#define bn_div 	PREFIX(bn_div)
#define bn_div_rem 	PREFIX(bn_div_rem)
#define bn_div_dig 	PREFIX(bn_div_dig)
#define bn_div_rem_dig 	PREFIX(bn_div_rem_dig)
#define bn_mod_2b 	PREFIX(bn_mod_2b)
#define bn_mod_dig 	PREFIX(bn_mod_dig)
#define bn_mod_basic 	PREFIX(bn_mod_basic)
#define bn_mod_pre_barrt 	PREFIX(bn_mod_pre_barrt)
#define bn_mod_barrt 	PREFIX(bn_mod_barrt)
#define bn_mod_pre_monty 	PREFIX(bn_mod_pre_monty)
#define bn_mod_monty_conv 	PREFIX(bn_mod_monty_conv)
#define bn_mod_monty_back 	PREFIX(bn_mod_monty_back)
#define bn_mod_monty_basic 	PREFIX(bn_mod_monty_basic)
#define bn_mod_monty_comba 	PREFIX(bn_mod_monty_comba)
#define bn_mod_pre_pmers 	PREFIX(bn_mod_pre_pmers)
#define bn_mod_pmers 	PREFIX(bn_mod_pmers)
#define bn_mxp_basic 	PREFIX(bn_mxp_basic)
#define bn_mxp_slide 	PREFIX(bn_mxp_slide)
#define bn_mxp_monty 	PREFIX(bn_mxp_monty)
#define bn_mxp_dig 	PREFIX(bn_mxp_dig)
#define bn_gcd_basic 	PREFIX(bn_gcd_basic)
#define bn_gcd_lehme 	PREFIX(bn_gcd_lehme)
#define bn_gcd_stein 	PREFIX(bn_gcd_stein)
#define bn_gcd_dig 	PREFIX(bn_gcd_dig)
#define bn_gcd_ext_basic 	PREFIX(bn_gcd_ext_basic)
#define bn_gcd_ext_lehme 	PREFIX(bn_gcd_ext_lehme)
#define bn_gcd_ext_stein 	PREFIX(bn_gcd_ext_stein)
#define bn_gcd_ext_mid 	PREFIX(bn_gcd_ext_mid)
#define bn_gcd_ext_dig 	PREFIX(bn_gcd_ext_dig)
#define bn_lcm 	PREFIX(bn_lcm)
#define bn_smb_leg 	PREFIX(bn_smb_leg)
#define bn_smb_jac 	PREFIX(bn_smb_jac)
#define bn_get_prime 	PREFIX(bn_get_prime)
#define bn_is_prime 	PREFIX(bn_is_prime)
#define bn_is_prime_basic 	PREFIX(bn_is_prime_basic)
#define bn_is_prime_rabin 	PREFIX(bn_is_prime_rabin)
#define bn_is_prime_solov 	PREFIX(bn_is_prime_solov)
#define bn_gen_prime_basic 	PREFIX(bn_gen_prime_basic)
#define bn_gen_prime_safep 	PREFIX(bn_gen_prime_safep)
#define bn_gen_prime_stron 	PREFIX(bn_gen_prime_stron)
#define bn_factor 	PREFIX(bn_factor)
#define bn_is_factor 	PREFIX(bn_is_factor)
#define bn_rec_win 	PREFIX(bn_rec_win)
#define bn_rec_slw 	PREFIX(bn_rec_slw)
#define bn_rec_naf 	PREFIX(bn_rec_naf)
#define bn_rec_tnaf 	PREFIX(bn_rec_tnaf)
#define bn_rec_reg 	PREFIX(bn_rec_reg)
#define bn_rec_jsf 	PREFIX(bn_rec_jsf)
#define bn_rec_glv 	PREFIX(bn_rec_glv)
#define bn_add1_low 	PREFIX(bn_add1_low)
#define bn_addn_low 	PREFIX(bn_addn_low)
#define bn_sub1_low 	PREFIX(bn_sub1_low)
#define bn_subn_low 	PREFIX(bn_subn_low)
#define bn_cmp1_low 	PREFIX(bn_cmp1_low)
#define bn_cmpn_low 	PREFIX(bn_cmpn_low)
#define bn_lsh1_low 	PREFIX(bn_lsh1_low)
#define bn_lshb_low 	PREFIX(bn_lshb_low)
#define bn_lshd_low 	PREFIX(bn_lshd_low)
#define bn_rsh1_low 	PREFIX(bn_rsh1_low)
#define bn_rshb_low 	PREFIX(bn_rshb_low)
#define bn_rshd_low 	PREFIX(bn_rshd_low)
#define bn_mula_low 	PREFIX(bn_mula_low)
#define bn_mul1_low 	PREFIX(bn_mul1_low)
#define bn_muln_low 	PREFIX(bn_muln_low)
#define bn_muld_low 	PREFIX(bn_muld_low)
#define bn_sqra_low 	PREFIX(bn_sqra_low)
#define bn_sqrn_low 	PREFIX(bn_sqrn_low)
#define bn_divn_low 	PREFIX(bn_divn_low)
#define bn_div1_low 	PREFIX(bn_div1_low)
#define bn_modn_low 	PREFIX(bn_modn_low)

#undef fp_st
#undef fp_t
#define fp_st	PREFIX(fp_st)
#define fp_t	PREFIX(fp_t)

#undef fp_prime_init
#undef fp_prime_clean
#undef fp_prime_get
#undef fp_prime_get_rdc
#undef fp_prime_get_conv
#undef fp_prime_get_mod8
#undef fp_prime_get_sps
#undef fp_prime_get_qnr
#undef fp_prime_get_cnr
#undef fp_param_get
#undef fp_prime_set_dense
#undef fp_prime_set_pmers
#undef fp_prime_calc
#undef fp_prime_conv
#undef fp_prime_conv_dig
#undef fp_prime_back
#undef fp_param_set
#undef fp_param_set_any
#undef fp_param_set_any_dense
#undef fp_param_set_any_pmers
#undef fp_param_set_any_tower
#undef fp_param_print
#undef fp_param_get_var
#undef fp_param_get_sps
#undef fp_param_get_map
#undef fp_copy
#undef fp_zero
#undef fp_is_zero
#undef fp_is_even
#undef fp_test_bit
#undef fp_get_bit
#undef fp_set_bit
#undef fp_set_dig
#undef fp_bits
#undef fp_rand
#undef fp_print
#undef fp_size
#undef fp_read
#undef fp_write
#undef fp_cmp
#undef fp_cmp_dig
#undef fp_add_basic
#undef fp_add_integ
#undef fp_add_dig
#undef fp_sub_basic
#undef fp_sub_integ
#undef fp_sub_dig
#undef fp_neg_basic
#undef fp_neg_integ
#undef fp_dbl_basic
#undef fp_dbl_integ
#undef fp_hlv_basic
#undef fp_hlv_integ
#undef fp_mul_basic
#undef fp_mul_comba
#undef fp_mul_integ
#undef fp_mul_karat
#undef fp_mul_dig
#undef fp_sqr_basic
#undef fp_sqr_comba
#undef fp_sqr_integ
#undef fp_sqr_karat
#undef fp_lsh
#undef fp_rsh
#undef fp_rdc_basic
#undef fp_rdc_monty_basic
#undef fp_rdc_monty_comba
#undef fp_rdc_quick
#undef fp_inv_basic
#undef fp_inv_binar
#undef fp_inv_monty
#undef fp_inv_exgcd
#undef fp_inv_lower
#undef fp_inv_sim
#undef fp_exp_basic
#undef fp_exp_slide
#undef fp_exp_monty
#undef fp_srt
#undef fp_add1_low
#undef fp_addn_low
#undef fp_addm_low
#undef fp_addd_low
#undef fp_addc_low
#undef fp_sub1_low
#undef fp_subn_low
#undef fp_subm_low
#undef fp_subd_low
#undef fp_subc_low
#undef fp_negm_low
#undef fp_dbln_low
#undef fp_dblm_low
#undef fp_hlvm_low
#undef fp_hlvd_low
#undef fp_cmp1_low
#undef fp_cmpn_low
#undef fp_lsh1_low
#undef fp_lshb_low
#undef fp_lshd_low
#undef fp_rsh1_low
#undef fp_rshb_low
#undef fp_rshd_low
#undef fp_mula_low
#undef fp_mul1_low
#undef fp_muln_low
#undef fp_mulm_low
#undef fp_sqrn_low
#undef fp_sqrm_low
#undef fp_rdcs_low
#undef fp_rdcn_low
#undef fp_invn_low

#define fp_prime_init 	PREFIX(fp_prime_init)
#define fp_prime_clean 	PREFIX(fp_prime_clean)
#define fp_prime_get 	PREFIX(fp_prime_get)
#define fp_prime_get_rdc 	PREFIX(fp_prime_get_rdc)
#define fp_prime_get_conv 	PREFIX(fp_prime_get_conv)
#define fp_prime_get_mod8 	PREFIX(fp_prime_get_mod8)
#define fp_prime_get_sps 	PREFIX(fp_prime_get_sps)
#define fp_prime_get_qnr 	PREFIX(fp_prime_get_qnr)
#define fp_prime_get_cnr 	PREFIX(fp_prime_get_cnr)
#define fp_param_get 	PREFIX(fp_param_get)
#define fp_prime_set_dense 	PREFIX(fp_prime_set_dense)
#define fp_prime_set_pmers 	PREFIX(fp_prime_set_pmers)
#define fp_prime_calc 	PREFIX(fp_prime_calc)
#define fp_prime_conv 	PREFIX(fp_prime_conv)
#define fp_prime_conv_dig 	PREFIX(fp_prime_conv_dig)
#define fp_prime_back 	PREFIX(fp_prime_back)
#define fp_param_set 	PREFIX(fp_param_set)
#define fp_param_set_any 	PREFIX(fp_param_set_any)
#define fp_param_set_any_dense 	PREFIX(fp_param_set_any_dense)
#define fp_param_set_any_pmers 	PREFIX(fp_param_set_any_pmers)
#define fp_param_set_any_tower 	PREFIX(fp_param_set_any_tower)
#define fp_param_print 	PREFIX(fp_param_print)
#define fp_param_get_var 	PREFIX(fp_param_get_var)
#define fp_param_get_sps 	PREFIX(fp_param_get_sps)
#define fp_param_get_map 	PREFIX(fp_param_get_map)
#define fp_copy 	PREFIX(fp_copy)
#define fp_zero 	PREFIX(fp_zero)
#define fp_is_zero 	PREFIX(fp_is_zero)
#define fp_is_even 	PREFIX(fp_is_even)
#define fp_test_bit 	PREFIX(fp_test_bit)
#define fp_get_bit 	PREFIX(fp_get_bit)
#define fp_set_bit 	PREFIX(fp_set_bit)
#define fp_set_dig 	PREFIX(fp_set_dig)
#define fp_bits 	PREFIX(fp_bits)
#define fp_rand 	PREFIX(fp_rand)
#define fp_print 	PREFIX(fp_print)
#define fp_size 	PREFIX(fp_size)
#define fp_read 	PREFIX(fp_read)
#define fp_write 	PREFIX(fp_write)
#define fp_cmp 	PREFIX(fp_cmp)
#define fp_cmp_dig 	PREFIX(fp_cmp_dig)
#define fp_add_basic 	PREFIX(fp_add_basic)
#define fp_add_integ 	PREFIX(fp_add_integ)
#define fp_add_dig 	PREFIX(fp_add_dig)
#define fp_sub_basic 	PREFIX(fp_sub_basic)
#define fp_sub_integ 	PREFIX(fp_sub_integ)
#define fp_sub_dig 	PREFIX(fp_sub_dig)
#define fp_neg_basic 	PREFIX(fp_neg_basic)
#define fp_neg_integ 	PREFIX(fp_neg_integ)
#define fp_dbl_basic 	PREFIX(fp_dbl_basic)
#define fp_dbl_integ 	PREFIX(fp_dbl_integ)
#define fp_hlv_basic 	PREFIX(fp_hlv_basic)
#define fp_hlv_integ 	PREFIX(fp_hlv_integ)
#define fp_mul_basic 	PREFIX(fp_mul_basic)
#define fp_mul_comba 	PREFIX(fp_mul_comba)
#define fp_mul_integ 	PREFIX(fp_mul_integ)
#define fp_mul_karat 	PREFIX(fp_mul_karat)
#define fp_mul_dig 	PREFIX(fp_mul_dig)
#define fp_sqr_basic 	PREFIX(fp_sqr_basic)
#define fp_sqr_comba 	PREFIX(fp_sqr_comba)
#define fp_sqr_integ 	PREFIX(fp_sqr_integ)
#define fp_sqr_karat 	PREFIX(fp_sqr_karat)
#define fp_lsh 	PREFIX(fp_lsh)
#define fp_rsh 	PREFIX(fp_rsh)
#define fp_rdc_basic 	PREFIX(fp_rdc_basic)
#define fp_rdc_monty_basic 	PREFIX(fp_rdc_monty_basic)
#define fp_rdc_monty_comba 	PREFIX(fp_rdc_monty_comba)
#define fp_rdc_quick 	PREFIX(fp_rdc_quick)
#define fp_inv_basic 	PREFIX(fp_inv_basic)
#define fp_inv_binar 	PREFIX(fp_inv_binar)
#define fp_inv_monty 	PREFIX(fp_inv_monty)
#define fp_inv_exgcd 	PREFIX(fp_inv_exgcd)
#define fp_inv_lower 	PREFIX(fp_inv_lower)
#define fp_inv_sim 	PREFIX(fp_inv_sim)
#define fp_exp_basic 	PREFIX(fp_exp_basic)
#define fp_exp_slide 	PREFIX(fp_exp_slide)
#define fp_exp_monty 	PREFIX(fp_exp_monty)
#define fp_srt 	PREFIX(fp_srt)
#define fp_add1_low 	PREFIX(fp_add1_low)
#define fp_addn_low 	PREFIX(fp_addn_low)
#define fp_addm_low 	PREFIX(fp_addm_low)
#define fp_addd_low 	PREFIX(fp_addd_low)
#define fp_addc_low 	PREFIX(fp_addc_low)
#define fp_sub1_low 	PREFIX(fp_sub1_low)
#define fp_subn_low 	PREFIX(fp_subn_low)
#define fp_subm_low 	PREFIX(fp_subm_low)
#define fp_subd_low 	PREFIX(fp_subd_low)
#define fp_subc_low 	PREFIX(fp_subc_low)
#define fp_negm_low 	PREFIX(fp_negm_low)
#define fp_dbln_low 	PREFIX(fp_dbln_low)
#define fp_dblm_low 	PREFIX(fp_dblm_low)
#define fp_hlvm_low 	PREFIX(fp_hlvm_low)
#define fp_hlvd_low 	PREFIX(fp_hlvd_low)
#define fp_cmp1_low 	PREFIX(fp_cmp1_low)
#define fp_cmpn_low 	PREFIX(fp_cmpn_low)
#define fp_lsh1_low 	PREFIX(fp_lsh1_low)
#define fp_lshb_low 	PREFIX(fp_lshb_low)
#define fp_lshd_low 	PREFIX(fp_lshd_low)
#define fp_rsh1_low 	PREFIX(fp_rsh1_low)
#define fp_rshb_low 	PREFIX(fp_rshb_low)
#define fp_rshd_low 	PREFIX(fp_rshd_low)
#define fp_mula_low 	PREFIX(fp_mula_low)
#define fp_mul1_low 	PREFIX(fp_mul1_low)
#define fp_muln_low 	PREFIX(fp_muln_low)
#define fp_mulm_low 	PREFIX(fp_mulm_low)
#define fp_sqrn_low 	PREFIX(fp_sqrn_low)
#define fp_sqrm_low 	PREFIX(fp_sqrm_low)
#define fp_rdcs_low 	PREFIX(fp_rdcs_low)
#define fp_rdcn_low 	PREFIX(fp_rdcn_low)
#define fp_invn_low 	PREFIX(fp_invn_low)

#undef fp_st
#undef fp_t
#define fp_st	PREFIX(fp_st)
#define fp_t	PREFIX(fp_t)

#undef fb_poly_init
#undef fb_poly_clean
#undef fb_poly_get
#undef fb_poly_set_dense
#undef fb_poly_set_trino
#undef fb_poly_set_penta
#undef fb_poly_get_srz
#undef fb_poly_tab_srz
#undef fb_poly_tab_sqr
#undef fb_poly_get_chain
#undef fb_poly_get_rdc
#undef fb_poly_get_trc
#undef fb_poly_get_slv
#undef fb_param_set
#undef fb_param_set_any
#undef fb_param_print
#undef fb_poly_add
#undef fb_poly_sub
#undef fb_copy
#undef fb_neg
#undef fb_zero
#undef fb_is_zero
#undef fb_test_bit
#undef fb_get_bit
#undef fb_set_bit
#undef fb_set_dig
#undef fb_bits
#undef fb_rand
#undef fb_print
#undef fb_size
#undef fb_read
#undef fb_write
#undef fb_cmp
#undef fb_cmp_dig
#undef fb_add
#undef fb_add_dig
#undef fb_sub
#undef fb_sub_dig
#undef fb_mul_basic
#undef fb_mul_integ
#undef fb_mul_lcomb
#undef fb_mul_rcomb
#undef fb_mul_lodah
#undef fb_mul_dig
#undef fb_mul_karat
#undef fb_sqr_basic
#undef fb_sqr_integ
#undef fb_sqr_table
#undef fb_lsh
#undef fb_rsh
#undef fb_rdc_basic
#undef fb_rdc_quick
#undef fb_srt_basic
#undef fb_srt_quick
#undef fb_trc_basic
#undef fb_trc_quick
#undef fb_inv_basic
#undef fb_inv_binar
#undef fb_inv_exgcd
#undef fb_inv_almos
#undef fb_inv_itoht
#undef fb_inv_bruch
#undef fb_inv_lower
#undef fb_inv_sim
#undef fb_exp_2b
#undef fb_exp_basic
#undef fb_exp_slide
#undef fb_exp_monty
#undef fb_slv_basic
#undef fb_slv_quick
#undef fb_itr_basic
#undef fb_itr_pre_quick
#undef fb_itr_quick
#undef fb_add1_low
#undef fb_addn_low
#undef fb_addd_low
#undef fb_cmp1_low
#undef fb_cmpn_low
#undef fb_lsh1_low
#undef fb_lshb_low
#undef fb_lshd_low
#undef fb_rsh1_low
#undef fb_rshb_low
#undef fb_rshd_low
#undef fb_lshadd_low
#undef fb_mul1_low
#undef fb_muln_low
#undef fb_muld_low
#undef fb_mulm_low
#undef fb_sqrn_low
#undef fb_sqrl_low
#undef fb_sqrm_low
#undef fb_itrn_low
#undef fb_srtn_low
#undef fb_slvn_low
#undef fb_trcn_low
#undef fb_rdcn_low
#undef fb_rdc1_low
#undef fb_invn_low

#define fb_poly_init 	PREFIX(fb_poly_init)
#define fb_poly_clean 	PREFIX(fb_poly_clean)
#define fb_poly_get 	PREFIX(fb_poly_get)
#define fb_poly_set_dense 	PREFIX(fb_poly_set_dense)
#define fb_poly_set_trino 	PREFIX(fb_poly_set_trino)
#define fb_poly_set_penta 	PREFIX(fb_poly_set_penta)
#define fb_poly_get_srz 	PREFIX(fb_poly_get_srz)
#define fb_poly_tab_srz 	PREFIX(fb_poly_tab_srz)
#define fb_poly_tab_sqr 	PREFIX(fb_poly_tab_sqr)
#define fb_poly_get_chain 	PREFIX(fb_poly_get_chain)
#define fb_poly_get_rdc 	PREFIX(fb_poly_get_rdc)
#define fb_poly_get_trc 	PREFIX(fb_poly_get_trc)
#define fb_poly_get_slv 	PREFIX(fb_poly_get_slv)
#define fb_param_set 	PREFIX(fb_param_set)
#define fb_param_set_any 	PREFIX(fb_param_set_any)
#define fb_param_print 	PREFIX(fb_param_print)
#define fb_poly_add 	PREFIX(fb_poly_add)
#define fb_poly_sub 	PREFIX(fb_poly_sub)
#define fb_copy 	PREFIX(fb_copy)
#define fb_neg 	PREFIX(fb_neg)
#define fb_zero 	PREFIX(fb_zero)
#define fb_is_zero 	PREFIX(fb_is_zero)
#define fb_test_bit 	PREFIX(fb_test_bit)
#define fb_get_bit 	PREFIX(fb_get_bit)
#define fb_set_bit 	PREFIX(fb_set_bit)
#define fb_set_dig 	PREFIX(fb_set_dig)
#define fb_bits 	PREFIX(fb_bits)
#define fb_rand 	PREFIX(fb_rand)
#define fb_print 	PREFIX(fb_print)
#define fb_size 	PREFIX(fb_size)
#define fb_read 	PREFIX(fb_read)
#define fb_write 	PREFIX(fb_write)
#define fb_cmp 	PREFIX(fb_cmp)
#define fb_cmp_dig 	PREFIX(fb_cmp_dig)
#define fb_add 	PREFIX(fb_add)
#define fb_add_dig 	PREFIX(fb_add_dig)
#define fb_sub 	PREFIX(fb_sub)
#define fb_sub_dig 	PREFIX(fb_sub_dig)
#define fb_mul_basic 	PREFIX(fb_mul_basic)
#define fb_mul_integ 	PREFIX(fb_mul_integ)
#define fb_mul_lcomb 	PREFIX(fb_mul_lcomb)
#define fb_mul_rcomb 	PREFIX(fb_mul_rcomb)
#define fb_mul_lodah 	PREFIX(fb_mul_lodah)
#define fb_mul_dig 	PREFIX(fb_mul_dig)
#define fb_mul_karat 	PREFIX(fb_mul_karat)
#define fb_sqr_basic 	PREFIX(fb_sqr_basic)
#define fb_sqr_integ 	PREFIX(fb_sqr_integ)
#define fb_sqr_table 	PREFIX(fb_sqr_table)
#define fb_lsh 	PREFIX(fb_lsh)
#define fb_rsh 	PREFIX(fb_rsh)
#define fb_rdc_basic 	PREFIX(fb_rdc_basic)
#define fb_rdc_quick 	PREFIX(fb_rdc_quick)
#define fb_srt_basic 	PREFIX(fb_srt_basic)
#define fb_srt_quick 	PREFIX(fb_srt_quick)
#define fb_trc_basic 	PREFIX(fb_trc_basic)
#define fb_trc_quick 	PREFIX(fb_trc_quick)
#define fb_inv_basic 	PREFIX(fb_inv_basic)
#define fb_inv_binar 	PREFIX(fb_inv_binar)
#define fb_inv_exgcd 	PREFIX(fb_inv_exgcd)
#define fb_inv_almos 	PREFIX(fb_inv_almos)
#define fb_inv_itoht 	PREFIX(fb_inv_itoht)
#define fb_inv_bruch 	PREFIX(fb_inv_bruch)
#define fb_inv_lower 	PREFIX(fb_inv_lower)
#define fb_inv_sim 	PREFIX(fb_inv_sim)
#define fb_exp_2b 	PREFIX(fb_exp_2b)
#define fb_exp_basic 	PREFIX(fb_exp_basic)
#define fb_exp_slide 	PREFIX(fb_exp_slide)
#define fb_exp_monty 	PREFIX(fb_exp_monty)
#define fb_slv_basic 	PREFIX(fb_slv_basic)
#define fb_slv_quick 	PREFIX(fb_slv_quick)
#define fb_itr_basic 	PREFIX(fb_itr_basic)
#define fb_itr_pre_quick 	PREFIX(fb_itr_pre_quick)
#define fb_itr_quick 	PREFIX(fb_itr_quick)
#define fb_add1_low 	PREFIX(fb_add1_low)
#define fb_addn_low 	PREFIX(fb_addn_low)
#define fb_addd_low 	PREFIX(fb_addd_low)
#define fb_cmp1_low 	PREFIX(fb_cmp1_low)
#define fb_cmpn_low 	PREFIX(fb_cmpn_low)
#define fb_lsh1_low 	PREFIX(fb_lsh1_low)
#define fb_lshb_low 	PREFIX(fb_lshb_low)
#define fb_lshd_low 	PREFIX(fb_lshd_low)
#define fb_rsh1_low 	PREFIX(fb_rsh1_low)
#define fb_rshb_low 	PREFIX(fb_rshb_low)
#define fb_rshd_low 	PREFIX(fb_rshd_low)
#define fb_lshadd_low 	PREFIX(fb_lshadd_low)
#define fb_mul1_low 	PREFIX(fb_mul1_low)
#define fb_muln_low 	PREFIX(fb_muln_low)
#define fb_muld_low 	PREFIX(fb_muld_low)
#define fb_mulm_low 	PREFIX(fb_mulm_low)
#define fb_sqrn_low 	PREFIX(fb_sqrn_low)
#define fb_sqrl_low 	PREFIX(fb_sqrl_low)
#define fb_sqrm_low 	PREFIX(fb_sqrm_low)
#define fb_itrn_low 	PREFIX(fb_itrn_low)
#define fb_srtn_low 	PREFIX(fb_srtn_low)
#define fb_slvn_low 	PREFIX(fb_slvn_low)
#define fb_trcn_low 	PREFIX(fb_trcn_low)
#define fb_rdcn_low 	PREFIX(fb_rdcn_low)
#define fb_rdc1_low 	PREFIX(fb_rdc1_low)
#define fb_invn_low 	PREFIX(fb_invn_low)

#undef ep_st
#undef ep_t
#define ep_st	PREFIX(ep_st)
#define ep_t	PREFIX(ep_t)

#undef eb_curve_init
#undef eb_curve_clean
#undef eb_curve_get_a
#undef eb_curve_get_b
#undef eb_curve_get_c
#undef eb_curve_opt_a
#undef eb_curve_opt_b
#undef eb_curve_opt_c
#undef eb_curve_is_kbltz
#undef eb_curve_is_super
#undef eb_curve_get_gen
#undef eb_curve_get_tab
#undef eb_curve_get_ord
#undef eb_curve_get_cof
#undef eb_curve_get_vm
#undef eb_curve_get_s0
#undef eb_curve_get_s1
#undef eb_curve_set_ordin
#undef eb_curve_set_kbltz
#undef eb_curve_set_super
#undef eb_param_set
#undef eb_param_set_any
#undef eb_param_set_any_ordin
#undef eb_param_set_any_kbltz
#undef eb_param_set_any_super
#undef eb_param_get
#undef eb_param_print
#undef eb_param_level
#undef eb_is_infty
#undef eb_set_infty
#undef eb_copy
#undef eb_cmp
#undef eb_rand
#undef eb_rhs
#undef eb_is_valid
#undef eb_tab
#undef eb_print
#undef eb_neg_basic
#undef eb_neg_projc
#undef eb_add_basic
#undef eb_add_projc
#undef eb_sub_basic
#undef eb_sub_projc
#undef eb_dbl_basic
#undef eb_dbl_projc
#undef eb_hlv
#undef eb_frb_basic
#undef eb_frb_projc
#undef eb_mul_basic
#undef eb_mul_lodah
#undef eb_mul_lwnaf
#undef eb_mul_rwnaf
#undef eb_mul_halve
#undef eb_mul_gen
#undef eb_mul_dig
#undef eb_mul_pre_basic
#undef eb_mul_pre_yaowi
#undef eb_mul_pre_nafwi
#undef eb_mul_pre_combs
#undef eb_mul_pre_combd
#undef eb_mul_pre_lwnaf
#undef eb_mul_fix_basic
#undef eb_mul_fix_yaowi
#undef eb_mul_fix_nafwi
#undef eb_mul_fix_combs
#undef eb_mul_fix_combd
#undef eb_mul_fix_lwnaf
#undef eb_mul_sim_basic
#undef eb_mul_sim_trick
#undef eb_mul_sim_inter
#undef eb_mul_sim_joint
#undef eb_mul_sim_gen
#undef eb_norm
#undef eb_norm_sim
#undef eb_map
#undef eb_pck
#undef eb_upk

#define eb_curve_init 	PREFIX(eb_curve_init)
#define eb_curve_clean 	PREFIX(eb_curve_clean)
#define eb_curve_get_a 	PREFIX(eb_curve_get_a)
#define eb_curve_get_b 	PREFIX(eb_curve_get_b)
#define eb_curve_get_c 	PREFIX(eb_curve_get_c)
#define eb_curve_opt_a 	PREFIX(eb_curve_opt_a)
#define eb_curve_opt_b 	PREFIX(eb_curve_opt_b)
#define eb_curve_opt_c 	PREFIX(eb_curve_opt_c)
#define eb_curve_is_kbltz 	PREFIX(eb_curve_is_kbltz)
#define eb_curve_is_super 	PREFIX(eb_curve_is_super)
#define eb_curve_get_gen 	PREFIX(eb_curve_get_gen)
#define eb_curve_get_tab 	PREFIX(eb_curve_get_tab)
#define eb_curve_get_ord 	PREFIX(eb_curve_get_ord)
#define eb_curve_get_cof 	PREFIX(eb_curve_get_cof)
#define eb_curve_get_vm 	PREFIX(eb_curve_get_vm)
#define eb_curve_get_s0 	PREFIX(eb_curve_get_s0)
#define eb_curve_get_s1 	PREFIX(eb_curve_get_s1)
#define eb_curve_set_ordin 	PREFIX(eb_curve_set_ordin)
#define eb_curve_set_kbltz 	PREFIX(eb_curve_set_kbltz)
#define eb_curve_set_super 	PREFIX(eb_curve_set_super)
#define eb_param_set 	PREFIX(eb_param_set)
#define eb_param_set_any 	PREFIX(eb_param_set_any)
#define eb_param_set_any_ordin 	PREFIX(eb_param_set_any_ordin)
#define eb_param_set_any_kbltz 	PREFIX(eb_param_set_any_kbltz)
#define eb_param_set_any_super 	PREFIX(eb_param_set_any_super)
#define eb_param_get 	PREFIX(eb_param_get)
#define eb_param_print 	PREFIX(eb_param_print)
#define eb_param_level 	PREFIX(eb_param_level)
#define eb_is_infty 	PREFIX(eb_is_infty)
#define eb_set_infty 	PREFIX(eb_set_infty)
#define eb_copy 	PREFIX(eb_copy)
#define eb_cmp 	PREFIX(eb_cmp)
#define eb_rand 	PREFIX(eb_rand)
#define eb_rhs 	PREFIX(eb_rhs)
#define eb_is_valid 	PREFIX(eb_is_valid)
#define eb_tab 	PREFIX(eb_tab)
#define eb_print 	PREFIX(eb_print)
#define eb_neg_basic 	PREFIX(eb_neg_basic)
#define eb_neg_projc 	PREFIX(eb_neg_projc)
#define eb_add_basic 	PREFIX(eb_add_basic)
#define eb_add_projc 	PREFIX(eb_add_projc)
#define eb_sub_basic 	PREFIX(eb_sub_basic)
#define eb_sub_projc 	PREFIX(eb_sub_projc)
#define eb_dbl_basic 	PREFIX(eb_dbl_basic)
#define eb_dbl_projc 	PREFIX(eb_dbl_projc)
#define eb_hlv 	PREFIX(eb_hlv)
#define eb_frb_basic 	PREFIX(eb_frb_basic)
#define eb_frb_projc 	PREFIX(eb_frb_projc)
#define eb_mul_basic 	PREFIX(eb_mul_basic)
#define eb_mul_lodah 	PREFIX(eb_mul_lodah)
#define eb_mul_lwnaf 	PREFIX(eb_mul_lwnaf)
#define eb_mul_rwnaf 	PREFIX(eb_mul_rwnaf)
#define eb_mul_halve 	PREFIX(eb_mul_halve)
#define eb_mul_gen 	PREFIX(eb_mul_gen)
#define eb_mul_dig 	PREFIX(eb_mul_dig)
#define eb_mul_pre_basic 	PREFIX(eb_mul_pre_basic)
#define eb_mul_pre_yaowi 	PREFIX(eb_mul_pre_yaowi)
#define eb_mul_pre_nafwi 	PREFIX(eb_mul_pre_nafwi)
#define eb_mul_pre_combs 	PREFIX(eb_mul_pre_combs)
#define eb_mul_pre_combd 	PREFIX(eb_mul_pre_combd)
#define eb_mul_pre_lwnaf 	PREFIX(eb_mul_pre_lwnaf)
#define eb_mul_fix_basic 	PREFIX(eb_mul_fix_basic)
#define eb_mul_fix_yaowi 	PREFIX(eb_mul_fix_yaowi)
#define eb_mul_fix_nafwi 	PREFIX(eb_mul_fix_nafwi)
#define eb_mul_fix_combs 	PREFIX(eb_mul_fix_combs)
#define eb_mul_fix_combd 	PREFIX(eb_mul_fix_combd)
#define eb_mul_fix_lwnaf 	PREFIX(eb_mul_fix_lwnaf)
#define eb_mul_sim_basic 	PREFIX(eb_mul_sim_basic)
#define eb_mul_sim_trick 	PREFIX(eb_mul_sim_trick)
#define eb_mul_sim_inter 	PREFIX(eb_mul_sim_inter)
#define eb_mul_sim_joint 	PREFIX(eb_mul_sim_joint)
#define eb_mul_sim_gen 	PREFIX(eb_mul_sim_gen)
#define eb_norm 	PREFIX(eb_norm)
#define eb_norm_sim 	PREFIX(eb_norm_sim)
#define eb_map 	PREFIX(eb_map)
#define eb_pck 	PREFIX(eb_pck)
#define eb_upk 	PREFIX(eb_upk)

#undef eb_st
#undef eb_t
#define eb_st	PREFIX(eb_st)
#define eb_t	PREFIX(eb_t)

#undef eb_curve_init
#undef eb_curve_clean
#undef eb_curve_get_a
#undef eb_curve_get_b
#undef eb_curve_get_c
#undef eb_curve_opt_a
#undef eb_curve_opt_b
#undef eb_curve_opt_c
#undef eb_curve_is_kbltz
#undef eb_curve_is_super
#undef eb_curve_get_gen
#undef eb_curve_get_tab
#undef eb_curve_get_ord
#undef eb_curve_get_cof
#undef eb_curve_get_vm
#undef eb_curve_get_s0
#undef eb_curve_get_s1
#undef eb_curve_set_ordin
#undef eb_curve_set_kbltz
#undef eb_curve_set_super
#undef eb_param_set
#undef eb_param_set_any
#undef eb_param_set_any_ordin
#undef eb_param_set_any_kbltz
#undef eb_param_set_any_super
#undef eb_param_get
#undef eb_param_print
#undef eb_param_level
#undef eb_is_infty
#undef eb_set_infty
#undef eb_copy
#undef eb_cmp
#undef eb_rand
#undef eb_rhs
#undef eb_is_valid
#undef eb_tab
#undef eb_print
#undef eb_neg_basic
#undef eb_neg_projc
#undef eb_add_basic
#undef eb_add_projc
#undef eb_sub_basic
#undef eb_sub_projc
#undef eb_dbl_basic
#undef eb_dbl_projc
#undef eb_hlv
#undef eb_frb_basic
#undef eb_frb_projc
#undef eb_mul_basic
#undef eb_mul_lodah
#undef eb_mul_lwnaf
#undef eb_mul_rwnaf
#undef eb_mul_halve
#undef eb_mul_gen
#undef eb_mul_dig
#undef eb_mul_pre_basic
#undef eb_mul_pre_yaowi
#undef eb_mul_pre_nafwi
#undef eb_mul_pre_combs
#undef eb_mul_pre_combd
#undef eb_mul_pre_lwnaf
#undef eb_mul_fix_basic
#undef eb_mul_fix_yaowi
#undef eb_mul_fix_nafwi
#undef eb_mul_fix_combs
#undef eb_mul_fix_combd
#undef eb_mul_fix_lwnaf
#undef eb_mul_sim_basic
#undef eb_mul_sim_trick
#undef eb_mul_sim_inter
#undef eb_mul_sim_joint
#undef eb_mul_sim_gen
#undef eb_norm
#undef eb_norm_sim
#undef eb_map
#undef eb_pck
#undef eb_upk

#define eb_curve_init 	PREFIX(eb_curve_init)
#define eb_curve_clean 	PREFIX(eb_curve_clean)
#define eb_curve_get_a 	PREFIX(eb_curve_get_a)
#define eb_curve_get_b 	PREFIX(eb_curve_get_b)
#define eb_curve_get_c 	PREFIX(eb_curve_get_c)
#define eb_curve_opt_a 	PREFIX(eb_curve_opt_a)
#define eb_curve_opt_b 	PREFIX(eb_curve_opt_b)
#define eb_curve_opt_c 	PREFIX(eb_curve_opt_c)
#define eb_curve_is_kbltz 	PREFIX(eb_curve_is_kbltz)
#define eb_curve_is_super 	PREFIX(eb_curve_is_super)
#define eb_curve_get_gen 	PREFIX(eb_curve_get_gen)
#define eb_curve_get_tab 	PREFIX(eb_curve_get_tab)
#define eb_curve_get_ord 	PREFIX(eb_curve_get_ord)
#define eb_curve_get_cof 	PREFIX(eb_curve_get_cof)
#define eb_curve_get_vm 	PREFIX(eb_curve_get_vm)
#define eb_curve_get_s0 	PREFIX(eb_curve_get_s0)
#define eb_curve_get_s1 	PREFIX(eb_curve_get_s1)
#define eb_curve_set_ordin 	PREFIX(eb_curve_set_ordin)
#define eb_curve_set_kbltz 	PREFIX(eb_curve_set_kbltz)
#define eb_curve_set_super 	PREFIX(eb_curve_set_super)
#define eb_param_set 	PREFIX(eb_param_set)
#define eb_param_set_any 	PREFIX(eb_param_set_any)
#define eb_param_set_any_ordin 	PREFIX(eb_param_set_any_ordin)
#define eb_param_set_any_kbltz 	PREFIX(eb_param_set_any_kbltz)
#define eb_param_set_any_super 	PREFIX(eb_param_set_any_super)
#define eb_param_get 	PREFIX(eb_param_get)
#define eb_param_print 	PREFIX(eb_param_print)
#define eb_param_level 	PREFIX(eb_param_level)
#define eb_is_infty 	PREFIX(eb_is_infty)
#define eb_set_infty 	PREFIX(eb_set_infty)
#define eb_copy 	PREFIX(eb_copy)
#define eb_cmp 	PREFIX(eb_cmp)
#define eb_rand 	PREFIX(eb_rand)
#define eb_rhs 	PREFIX(eb_rhs)
#define eb_is_valid 	PREFIX(eb_is_valid)
#define eb_tab 	PREFIX(eb_tab)
#define eb_print 	PREFIX(eb_print)
#define eb_neg_basic 	PREFIX(eb_neg_basic)
#define eb_neg_projc 	PREFIX(eb_neg_projc)
#define eb_add_basic 	PREFIX(eb_add_basic)
#define eb_add_projc 	PREFIX(eb_add_projc)
#define eb_sub_basic 	PREFIX(eb_sub_basic)
#define eb_sub_projc 	PREFIX(eb_sub_projc)
#define eb_dbl_basic 	PREFIX(eb_dbl_basic)
#define eb_dbl_projc 	PREFIX(eb_dbl_projc)
#define eb_hlv 	PREFIX(eb_hlv)
#define eb_frb_basic 	PREFIX(eb_frb_basic)
#define eb_frb_projc 	PREFIX(eb_frb_projc)
#define eb_mul_basic 	PREFIX(eb_mul_basic)
#define eb_mul_lodah 	PREFIX(eb_mul_lodah)
#define eb_mul_lwnaf 	PREFIX(eb_mul_lwnaf)
#define eb_mul_rwnaf 	PREFIX(eb_mul_rwnaf)
#define eb_mul_halve 	PREFIX(eb_mul_halve)
#define eb_mul_gen 	PREFIX(eb_mul_gen)
#define eb_mul_dig 	PREFIX(eb_mul_dig)
#define eb_mul_pre_basic 	PREFIX(eb_mul_pre_basic)
#define eb_mul_pre_yaowi 	PREFIX(eb_mul_pre_yaowi)
#define eb_mul_pre_nafwi 	PREFIX(eb_mul_pre_nafwi)
#define eb_mul_pre_combs 	PREFIX(eb_mul_pre_combs)
#define eb_mul_pre_combd 	PREFIX(eb_mul_pre_combd)
#define eb_mul_pre_lwnaf 	PREFIX(eb_mul_pre_lwnaf)
#define eb_mul_fix_basic 	PREFIX(eb_mul_fix_basic)
#define eb_mul_fix_yaowi 	PREFIX(eb_mul_fix_yaowi)
#define eb_mul_fix_nafwi 	PREFIX(eb_mul_fix_nafwi)
#define eb_mul_fix_combs 	PREFIX(eb_mul_fix_combs)
#define eb_mul_fix_combd 	PREFIX(eb_mul_fix_combd)
#define eb_mul_fix_lwnaf 	PREFIX(eb_mul_fix_lwnaf)
#define eb_mul_sim_basic 	PREFIX(eb_mul_sim_basic)
#define eb_mul_sim_trick 	PREFIX(eb_mul_sim_trick)
#define eb_mul_sim_inter 	PREFIX(eb_mul_sim_inter)
#define eb_mul_sim_joint 	PREFIX(eb_mul_sim_joint)
#define eb_mul_sim_gen 	PREFIX(eb_mul_sim_gen)
#define eb_norm 	PREFIX(eb_norm)
#define eb_norm_sim 	PREFIX(eb_norm_sim)
#define eb_map 	PREFIX(eb_map)
#define eb_pck 	PREFIX(eb_pck)
#define eb_upk 	PREFIX(eb_upk)

#endif /* LABEL */

#endif /* !RELIC_LABEL_H */
