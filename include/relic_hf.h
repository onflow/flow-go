/*
 * Copyright 2007 Project RELIC
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file.
 *
 * RELIC is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * RELIC is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with RELIC. If not, see <http://www.gnu.org/licenses/>.
 */

/**
 * @defgroup hf Hash functions.
 */

/**
 * @file
 *
 * Interface of the hash functions module.
 *
 * @version $Id$
 * @ingroup hf
 */

#ifndef RELIC_HF_H
#define RELIC_HF_H

#include "relic_conf.h"
#include "relic_types.h"

/**
 * Maps a byte vector to a fixed-length byte vector using the chosen hash
 * function.
 *
 * @param[out] H				- the digest.
 * @param[in] M					- the message to hash.
 * @param[in] L					- the message length in bytes.
 */
#if HF_MAP == SHONE
#define hf_map(H, M, L)			hf_map_shone(H, M, L)
#elif HF_MAP == SH224
#define hf_map(H, M, L)			hf_map_sh224(H, M, L)
#elif HF_MAP == SH256
#define hf_map(H, M, L)			hf_map_sh256(H, M, L)
#elif HF_MAP == SH384
#define hf_map(H, M, L)			hf_map_sh384(H, M, L)
#elif HF_MAP == SH512
#define hf_map(H, M, L)			hf_map_sh512(H, M, L)
#endif

/**
 * Length in bytes of hash function output.
 */
#if HF_MAP == SHONE
#define HF_LEN					20
#elif HF_MAP == SH224
#define HF_LEN					28
#elif HF_MAP == SH256
#define HF_LEN					32
#elif HF_MAP == SH384
#define HF_LEN					48
#elif HF_MAP == SH512
#define HF_LEN					64
#endif

/**
 * Computes the SHA-1 hash function.
 *
 * @param[out] hash				- the digest.
 * @param[in] msg				- the message to hash.
 * @param[in] len				- the message length in bytes.
 */
void hf_map_shone(unsigned char *hash, unsigned char *msg, int len);

/**
 * Initializes the hash function context.
 */
void hf_sha1_init(void);

/**
 * Updates the hash function context with more data.
 *
 * @param[in] msg				- the message to hash.
 * @param[in] len				- the message length in bytes.
 */
void hf_sha1_update(unsigned char *msg, int len);

/**
 * Finalizes the hash function computation.
 *
 * @param[out] hash				- the digest.
 */
void hf_sha1_final(unsigned char *hash);

/**
 * Returns the internal state of the hash function.
 *
 * @param[out] state			- the internal state.
 */
void hf_sha1_state(unsigned char *state);

/**
 * Computes the SHA-224 hash function.
 *
 * @param[out] hash				- the digest.
 * @param[in] msg				- the message to hash.
 * @param[in] len				- the message length in bytes.
 */
void hf_map_sh224(unsigned char *hash, unsigned char *msg, int len);

/**
 * Initializes the hash function context.
 */
void hf_sha224_init(void);

/**
 * Updates the hash function context with more data.
 *
 * @param[in] msg				- the message to hash.
 * @param[in] len				- the message length in bytes.
 */
void hf_sha224_update(unsigned char *msg, int len);

/**
 * Finalizes the hash function computation.
 *
 * @param[out] hash				- the digest.
 */
void hf_sha224_final(unsigned char *hash);

/**
 * Returns the internal state of the hash function.
 *
 * @param[out] state			- the internal state.
 */
void hf_sha224_state(unsigned char *state);

/**
 * Computes the SHA-256 hash function.
 *
 * @param[out] hash				- the digest.
 * @param[in] msg				- the message to hash.
 * @param[in] len				- the message length in bytes.
 */
void hf_map_sh256(unsigned char *hash, unsigned char *msg, int len);

/**
 * Initializes the hash function context.
 */
void hf_sha256_init(void);

/**
 * Updates the hash function context with more data.
 *
 * @param[in] msg				- the message to hash.
 * @param[in] len				- the message length in bytes.
 */
void hf_sha256_update(unsigned char *msg, int len);

/**
 * Finalizes the hash function computation.
 *
 * @param[out] hash				- the digest.
 */
void hf_sha256_final(unsigned char *hash);

/**
 * Returns the internal state of the hash function.
 *
 * @param[out] state			- the internal state.
 */
void hf_sha256_state(unsigned char *state);

/**
 * Computes the SHA-384 hash function.
 *
 * @param[out] hash				- the digest.
 * @param[in] msg				- the message to hash.
 * @param[in] len				- the message length in bytes.
 */
void hf_map_sh384(unsigned char *hash, unsigned char *msg, int len);

/**
 * Initializes the hash function context.
 */
void hf_sha384_init(void);

/**
 * Updates the hash function context with more data.
 *
 * @param[in] msg				- the message to hash.
 * @param[in] len				- the message length in bytes.
 */
void hf_sha384_update(unsigned char *msg, int len);

/**
 * Finalizes the hash function computation.
 *
 * @param[out] hash				- the digest.
 */
void hf_sha384_final(unsigned char *hash);

/**
 * Returns the internal state of the hash function.
 *
 * @param[out] state			- the internal state.
 */
void hf_sha384_state(unsigned char *state);

/**
 * Computes the SHA-512 hash function.
 *
 * @param[out] hash				- the digest.
 * @param[in] msg				- the message to hash.
 * @param[in] len				- the message length in bytes.
 */
void hf_map_sh512(unsigned char *hash, unsigned char *msg, int len);

/**
 * Initializes the hash function context.
 */
void hf_sha512_init(void);

/**
 * Updates the hash function context with more data.
 *
 * @param[in] msg				- the message to hash.
 * @param[in] len				- the message length in bytes.
 */
void hf_sha512_update(unsigned char *msg, int len);

/**
 * Finalizes the hash function computation.
 *
 * @param[out] hash				- the digest.
 */
void hf_sha512_final(unsigned char *hash);

/**
 * Returns the internal state of the hash function.
 *
 * @param[out] state			- the internal state.
 */
void hf_sha512_state(unsigned char *state);

#endif /* !RELIC_HF_H */
