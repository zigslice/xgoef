/* Linux amd64 runtime prefixed to an xgoef AppImage.
   Rebuild from this directory with:
   gcc -O2 -s -o ../appruntime.linux-amd64 appruntime.c
   The 16-byte marker is followed by the squashfs offset. */
#include <errno.h>
#include <fcntl.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <unistd.h>

struct __attribute__((packed)) locator_s {
	char magic[16];
	uint64_t offset;
};

static volatile struct locator_s locator = {
	{'X', 'G', 'O', 'E', 'F', '_', 'S', 'Q', '_', 'O', 'F', 'F', 'S', 'E', 'T', '!'},
	0};

static int fail(const char *msg) {
	fprintf(stderr, "xgoef-appimage: %s\n", msg);
	return 1;
}

static int pread_full(int fd, void *dst, size_t n, uint64_t at) {
	uint8_t *out = dst;
	size_t got = 0;
	while (got < n) {
		ssize_t nread = pread(fd, out + got, n - got, (off_t)(at + got));
		if (nread < 0) {
			if (errno == EINTR)
				continue;
			return -1;
		}
		if (nread == 0)
			return -1;
		got += (size_t)nread;
	}
	return 0;
}

static uint16_t ru16(const uint8_t *p) {
	return (uint16_t)(p[0] | (p[1] << 8));
}

static uint32_t ru32(const uint8_t *p) {
	return (uint32_t)p[0] | ((uint32_t)p[1] << 8) | ((uint32_t)p[2] << 16) | ((uint32_t)p[3] << 24);
}

static uint64_t ru64(const uint8_t *p) {
	return (uint64_t)ru32(p) | ((uint64_t)ru32(p + 4) << 32);
}

static int meta_read(int fd, uint64_t image, uint64_t table, uint32_t block, uint32_t offset, uint8_t *dst, uint32_t length) {
	uint64_t pos = image + table;
	for (uint32_t i = 0; i < block; i++) {
		uint8_t hdrb[2];
		if (pread_full(fd, hdrb, 2, pos) < 0)
			return -1;
		pos += 2u + (ru16(hdrb) & 0x7fffu);
	}
	uint32_t filled = 0;
	uint32_t skip = offset;
	while (filled < length) {
		uint8_t hdrb[2];
		if (pread_full(fd, hdrb, 2, pos) < 0)
			return -1;
		uint16_t hdr = ru16(hdrb);
		pos += 2;
		if ((hdr & 0x8000u) == 0)
			return -1;
		uint32_t sz = hdr & 0x7fffu;
		if (skip >= sz) {
			skip -= sz;
			pos += sz;
			continue;
		}
		uint32_t take = sz - skip;
		if (take > length - filled)
			take = length - filled;
		if (pread_full(fd, dst + filled, take, pos + skip) < 0)
			return -1;
		filled += take;
		pos += sz;
		skip = 0;
	}
	return 0;
}

static int mkdir_p(char *path) {
	for (char *p = path + 1; *p; p++) {
		if (*p != '/')
			continue;
		*p = 0;
		if (mkdir(path, 0755) < 0 && errno != EEXIST) {
			*p = '/';
			return -1;
		}
		*p = '/';
	}
	if (mkdir(path, 0755) < 0 && errno != EEXIST)
		return -1;
	return 0;
}

static uint32_t crc32(const uint8_t *p, size_t n) {
	uint32_t crc = 0xffffffffu;
	for (size_t i = 0; i < n; i++) {
		crc ^= p[i];
		for (int b = 0; b < 8; b++)
			crc = (crc >> 1) ^ (0xedb88320u & (0u - (crc & 1u)));
	}
	return ~crc;
}

struct image {
	int fd;
	uint64_t base;
	uint32_t block_size;
	uint64_t inode_table;
	uint64_t dir_table;
};

static int extract_ino(struct image *img, uint32_t block, uint32_t offset, const char *path);

static int extract_dir(struct image *img, const uint8_t *ino, const char *path) {
	uint32_t start = ru32(ino + 16);
	uint32_t file_size = ru16(ino + 24);
	uint16_t off = ru16(ino + 26);
	if (file_size < 3)
		return -1;
	uint32_t dirlen = file_size - 3;
	uint8_t *buf = malloc(dirlen ? dirlen : 1);
	if (!buf)
		return -1;
	if (meta_read(img->fd, img->base, img->dir_table, start, off, buf, dirlen) < 0) {
		free(buf);
		return -1;
	}
	if (mkdir_p((char *)path) < 0) {
		free(buf);
		return -1;
	}
	uint32_t pos = 0;
	int rc = 0;
	while (pos + 12 <= dirlen) {
		uint32_t count = ru32(buf + pos);
		uint32_t iblock = ru32(buf + pos + 4);
		uint32_t base_ino = ru32(buf + pos + 8);
		pos += 12;
		for (uint32_t n = 0; n < count + 1; n++) {
			if (pos + 8 > dirlen) {
				rc = -1;
				break;
			}
			uint16_t ioff = ru16(buf + pos);
			int16_t diff = (int16_t)ru16(buf + pos + 2);
			uint16_t namelen = (uint16_t)(ru16(buf + pos + 6) + 1u);
			pos += 8;
			if (namelen == 0 || pos + namelen > dirlen) {
				rc = -1;
				break;
			}
			char name[256];
			if (namelen >= sizeof name) {
				rc = -1;
				break;
			}
			memcpy(name, buf + pos, namelen);
			name[namelen] = 0;
			pos += namelen;
			if (strcmp(name, ".") == 0 || strcmp(name, "..") == 0 || strchr(name, '/')) {
				rc = -1;
				break;
			}
			char child[4096];
			if (snprintf(child, sizeof child, "%s/%s", path, name) >= (int)sizeof child) {
				rc = -1;
				break;
			}
			(void)base_ino;
			(void)diff;
			if (extract_ino(img, iblock, ioff, child) < 0) {
				rc = -1;
				break;
			}
		}
		if (rc < 0)
			break;
	}
	free(buf);
	return rc;
}

static int extract_ino(struct image *img, uint32_t block, uint32_t offset, const char *path) {
	uint8_t raw[8192];
	if (meta_read(img->fd, img->base, img->inode_table, block, offset, raw, 16) < 0)
		return -1;
	uint16_t kind = ru16(raw);
	uint16_t mode = ru16(raw + 2);
	if (kind == 1) {
		if (meta_read(img->fd, img->base, img->inode_table, block, offset, raw, 32) < 0)
			return -1;
		if (extract_dir(img, raw, path) < 0)
			return -1;
		chmod(path, mode & 0777);
		return 0;
	}
	if (kind == 3) {
		if (meta_read(img->fd, img->base, img->inode_table, block, offset, raw, 24) < 0)
			return -1;
		uint32_t n = ru32(raw + 20);
		if (n >= 4096)
			return -1;
		if (meta_read(img->fd, img->base, img->inode_table, block, offset, raw, 24 + n) < 0)
			return -1;
		char target[4096];
		memcpy(target, raw + 24, n);
		target[n] = 0;
		if (symlink(target, path) < 0 && errno != EEXIST)
			return -1;
		return 0;
	}
	if (kind != 2)
		return -1;
	if (meta_read(img->fd, img->base, img->inode_table, block, offset, raw, 32) < 0)
		return -1;
	uint32_t start = ru32(raw + 16);
	uint32_t fragment = ru32(raw + 20);
	uint32_t file_size = ru32(raw + 28);
	uint32_t blocks = 0;
	if (file_size > 0 && fragment == 0xffffffffu)
		blocks = (file_size + img->block_size - 1) / img->block_size;
	if (32u + blocks * 4u > sizeof raw)
		return -1;
	if (meta_read(img->fd, img->base, img->inode_table, block, offset, raw, 32 + blocks * 4) < 0)
		return -1;
	char parent[4096];
	snprintf(parent, sizeof parent, "%s", path);
	char *slash = strrchr(parent, '/');
	if (slash && slash != parent) {
		*slash = 0;
		if (mkdir_p(parent) < 0)
			return -1;
	}
	int out = open(path, O_CREAT | O_EXCL | O_WRONLY, 0644);
	if (out < 0)
		return -1;
	uint64_t at = img->base + start;
	uint32_t left = file_size;
	for (uint32_t i = 0; i < blocks; i++) {
		uint32_t blk = ru32(raw + 32 + i * 4);
		if ((blk & 0x01000000u) == 0) {
			close(out);
			return -1;
		}
		uint32_t sz = blk & 0x00ffffffu;
		uint32_t take = sz;
		if (take > left)
			take = left;
		uint8_t copy[65536];
		uint32_t done = 0;
		while (done < take) {
			uint32_t chunk = take - done;
			if (chunk > sizeof copy)
				chunk = sizeof copy;
			if (pread_full(img->fd, copy, chunk, at + done) < 0) {
				close(out);
				return -1;
			}
			uint32_t wrote = 0;
			while (wrote < chunk) {
				ssize_t n = write(out, copy + wrote, chunk - wrote);
				if (n < 0) {
					if (errno == EINTR)
						continue;
					close(out);
					return -1;
				}
				wrote += (uint32_t)n;
			}
			done += chunk;
		}
		at += sz;
		left -= take;
	}
	if (fchmod(out, mode & 0777) < 0) {
		close(out);
		return -1;
	}
	close(out);
	return 0;
}

int main(int argc, char **argv) {
	uint64_t sq = locator.offset;
	char self[4096];
	ssize_t nself = readlink("/proc/self/exe", self, sizeof self - 1);
	if (nself < 0)
		return fail("cannot read /proc/self/exe");
	self[nself] = 0;
	int fd = open(self, O_RDONLY);
	if (fd < 0)
		return fail("cannot open the AppImage");
	uint8_t super[96];
	if (pread_full(fd, super, sizeof super, sq) < 0 || ru32(super) != 0x73717368u) {
		close(fd);
		return fail("squashfs header is missing");
	}
	uint32_t bytes_used = ru32(super + 40);
	uint8_t *sumbuf = malloc(bytes_used > 1048576 ? 1048576 : (bytes_used ? bytes_used : 1));
	uint32_t sumlen = bytes_used > 1048576 ? 1048576 : bytes_used;
	if (!sumbuf || (sumlen && pread_full(fd, sumbuf, sumlen, sq) < 0)) {
		free(sumbuf);
		close(fd);
		return fail("cannot read the image");
	}
	uint32_t sum = crc32(sumbuf, sumlen);
	free(sumbuf);
	const char *cache = getenv("XDG_CACHE_HOME");
	char cachebuf[4096];
	if (!cache || !cache[0]) {
		const char *home = getenv("HOME");
		if (home && home[0]) {
			snprintf(cachebuf, sizeof cachebuf, "%s/.cache", home);
			cache = cachebuf;
		} else {
			cache = "/tmp";
		}
	}
	char dest[4096];
	if (snprintf(dest, sizeof dest, "%s/xgoef-appimage/%08x-%08x", cache, bytes_used, sum) >= (int)sizeof dest) {
		close(fd);
		return fail("cache path is too long");
	}
	char apprun[4096];
	snprintf(apprun, sizeof apprun, "%s/AppRun", dest);
	if (access(apprun, X_OK) != 0) {
		char tmp[4096];
		snprintf(tmp, sizeof tmp, "%s.tmp-%d", dest, (int)getpid());
		if (mkdir_p(tmp) < 0) {
			close(fd);
			return fail("cannot create the cache directory");
		}
		struct image img = {
			.fd = fd,
			.base = sq,
			.block_size = ru32(super + 12),
			.inode_table = ru64(super + 64),
			.dir_table = ru64(super + 72),
		};
		uint64_t root = ru64(super + 32);
		if (extract_ino(&img, (uint32_t)(root >> 16), (uint32_t)(root & 0xffffu), tmp) < 0) {
			close(fd);
			return fail("cannot extract the image");
		}
		if (rename(tmp, dest) < 0) {
			/* Another launch won the race. Use the directory that is there. */
			if (access(apprun, X_OK) != 0) {
				close(fd);
				return fail("cannot publish the cache directory");
			}
		}
	}
	close(fd);
	char **args = calloc((size_t)argc + 1, sizeof(char *));
	if (!args)
		return fail("out of memory");
	args[0] = apprun;
	for (int i = 1; i < argc; i++)
		args[i] = argv[i];
	execv(apprun, args);
	fprintf(stderr, "xgoef-appimage: cannot exec AppRun: %s\n", strerror(errno));
	return 1;
}
