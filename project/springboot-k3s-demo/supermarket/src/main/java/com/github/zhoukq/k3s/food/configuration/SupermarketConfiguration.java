package com.github.zhoukq.k3s.food.configuration;

import com.github.zhoukq.k3s.food.remote.DigitalRemote;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.http.client.JdkClientHttpRequestFactory;

import java.time.Duration;
import java.time.temporal.ChronoUnit;

@Configuration
public class SupermarketConfiguration {
    public static final Logger LOGGER = LoggerFactory.getLogger(SupermarketConfiguration.class);

    @Value("${remote.timeout}")
    public Long timeout;
    @Bean
    public JdkClientHttpRequestFactory jdkClientHttpRequestFactory(){
        LOGGER.info("Creating jdkClientHttpRequestFactory with timeout {}", timeout);
        JdkClientHttpRequestFactory requestFactory = new JdkClientHttpRequestFactory();
        requestFactory.setReadTimeout(Duration.of(timeout, ChronoUnit.MILLIS));
        return requestFactory;
    }
}
