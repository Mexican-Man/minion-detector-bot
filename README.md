# minion-detector-bot
Discord bot to detect minion (from Despicable Me) pictures/videos using AutoML, and delete them. Obviously it doesn't have to be minions; it can be literally anything (with a few tiny modifications).
Mileage may vary.

Run the AutoML model through GCR or on your own machine (the container runs an HTTP server), then run this bot which will interact with it over HTTP.

### Disclaimer
As it turns out, Google Cloud Run is monumentally cheaper than having Google host your model 24/7. For more info, [here's an article](https://medium.com/@juri.sarbach/how-to-deploy-your-automl-model-in-a-cost-effective-way-5efdd377d4d2)
that goes over that, and how to set it up. The article is a little bit old and vague, but I ended up getting it working. In the end, I opted to run it on a computer at my home (instead of running it on GCR),
and gave it a tiny GPU to speed up video transcoding. This project isn't set up to use a GPU, but can be configured really easily (see [here](https://www.tensorflow.org/install/gpu) and [here](https://stackoverflow.com/a/44518572/9731890)).

### How to set up
Prepare your training data [as instructed here](https://cloud.google.com/vision/automl/docs/prepare). You're going to need lots of minion stills, and even more pictures of literally anything that isn't a minion. Google lets you either keep a bucket of pictures and generated a CSV, or upload pictures from your computer each time.
I opted to do the bucket, as instructed in the doc. I made a folder, containing two folders like such:
```
training-data/
├─ None_of_the_above/
├─ minion/
├─ uploadtraining.sh
```
and make a `uploadtraining.sh` in said folder like this:
```bash
# Create csv file and define bucket path
bucket_path="gs://your-bucket-name"
filename="labels_csv_bash.csv"
touch $filename
IFS=$'\n' # Internal field separator variable has to be set to separate on new lines
# List of every .jpg file inside the buckets folder. ** searches for them recursively.
for i in `gsutil -m ls $bucket_path/minion/**`
do
        # Cuts the address using the / limiter and gets the second item starting from the end.
        label=$(echo $i | rev | cut -d'/' -f2 | rev)
        echo "$i, minion" >> $filename
done
for i in `gsutil -m ls $bucket_path/None_of_the_above/**`
do
        # Cuts the address using the / limiter and gets the second item starting from the end.
        label=$(echo $i | rev | cut -d'/' -f2 | rev)
        echo "$i, other" >> $filename
done
IFS=' '
gsutil -m cp $filename $bucket_path
```
> Note: you'll have to install and setup GSUTIL for this to work.

Read the [article listed at the top](https://medium.com/@juri.sarbach/how-to-deploy-your-automl-model-in-a-cost-effective-way-5efdd377d4d2) to learn how to export your model,
build your container, and optionally run it on Google Cloud Run. If you don't want to run it on GCR, just download the build Docker image to whichever machine you'd like, and run there.
When setting up your GCR routine, make sure NOT to tick "HTTP/2". That broke it for me, and gave me `HTTP 5XX` errors.

Download my code, input your bot token, insert the IP (internal or external, make sure to forward your ports), make any changes, and run it.

> Note: the code is set up for FFMPEG to use HTTPS, which it doesn't by default. Check [this](https://askubuntu.com/a/650617) to learn how to recompile it with OpenSSL.
> Alternatively, there's a commented out regex in the code to replace `HTTPS` with `HTTP`, but means you won't be able to get media from certain sites, including things uploaded directly to discord.

I've included a linked to one of my Docker images in the "Releases" tab. I probably won't update it, but this one works pretty decent (see below).

### How does it work?
It's really simple, the bot converts uploaded/embedded images to Base64, and sends them via HTTP to our TensorFlow "server". It sends us back a confidence value, and through that we can determine if our image contains a minion. If a user uploads/embeds a video/gifv/gif/whatever, the bot uses FFMPEG to grab a few frames (this is configurable, I set it pretty low to save performance), and uses those as individual images. I programmed this in Go, but since all of the heavylifting is done by FFMPEG and TensorFlow, the performance loss wouldn't be noticable using a scripted language.

### Conclusion
I'm pretty happy with this setup. Google's Vision web interface makes it really easy for someone to setup and training.
I ended up having to upload lots and lots of images to train, which took a very long time, and took many iterations of testing.
Unfortunately, Google only gives you 20 hours worth of training for free, meaning that this isn't a very good long term solution. As well,
not being able to control the type of algorithm used might have hindered the model's accuracy. The CNN does do a pretty decent job, but I can't help but feeling that
if I had build it all myself instead of relying on GCP, I might've had better luck (maybe using Object Detection instead of Image Classification). Not to mention, it'd be free to train.

That being said, it does do a surprisingly good job. After getting enough training data in, there were only a handful or false positives out of hundreds of test images.
Obviously "minions" as a concept can be pretty abstract, so you have to draw the line with the training data on what is allowed, such as: minion fanart, toys, articles of clothing, etc.
Even then, it fairs pretty well with the abstract stuff. However, in my case, it really didn't like Arthur, Yellow Powerrangers, and anything purple, for some reason.
