#!/usr/bin/env python3
# Synthetic test media, generated locally; MIT, like the application.
# Run with a development FFmpeg that includes lavfi, libvpx and libvorbis.
from pathlib import Path
import subprocess
root=Path(__file__).resolve().parent
def ff(*args): subprocess.run(['ffmpeg','-v','error','-y',*args],check=True)
video=['-f','lavfi','-i','testsrc2=size=160x94:rate=10:duration=1','-f','lavfi','-i','sine=frequency=440:duration=1','-map','0:v','-map','1:a']
for ext,codec in {'mkv':['-c:v','mpeg4','-c:a','pcm_s16le'],'mp4':['-c:v','mpeg4','-c:a','aac'],'avi':['-c:v','mpeg4','-c:a','libmp3lame'],'mov':['-c:v','mpeg4','-c:a','aac'],'webm':['-c:v','libvpx','-deadline','realtime','-cpu-used','8','-c:a','libvorbis']}.items():
 ff(*video,*codec,str(root/f'source.{ext}'))
for ext,codec in {'flac':'flac','wav':'pcm_s16le','mp3':'libmp3lame','m4a':'aac'}.items():
 ff('-f','lavfi','-i','sine=frequency=500:duration=1','-c:a',codec,str(root/f'source.{ext}'))
ff('-f','lavfi','-i','testsrc2=size=160x90:rate=10:duration=2','-f','lavfi','-i','sine=frequency=440:duration=2','-f','lavfi','-i','sine=frequency=880:duration=2','-map','0:v','-map','1:a','-map','2:a','-metadata:s:a:0','language=deu','-metadata:s:a:1','language=eng','-c:v','mpeg4','-c:a','pcm_s16le',str(root/'multi.mkv'))
ff('-f','lavfi','-i','sine=frequency=1200:duration=2','-c:a','pcm_s16le',str(root/'external.wav'))
print('Generated all synthetic fixtures.')
